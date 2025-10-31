package group

import (
	cfg "GoStacker/pkg/config"
	"GoStacker/pkg/db/mysql"
	rdb "GoStacker/pkg/db/redis"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	gredis "github.com/go-redis/redis"
)

func RunGroupFlusher(interval time.Duration, batchSize int, stopCh <-chan struct{}) {
	// determine retention from config (fallback to 7 days)
	retention := 7 * 24 * time.Hour
	if cfg.Conf != nil && cfg.Conf.GroupCacheConfig != nil && cfg.Conf.GroupCacheConfig.DirtyRetentionSeconds > 0 {
		retention = time.Duration(cfg.Conf.GroupCacheConfig.DirtyRetentionSeconds) * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			log.Println("group flusher stopping")
			return
		case <-ticker.C:
			// 清理超期的 dirty 元素，避免无限增长
			CleanStaleDirty(retention)
			// process groups
			processDirtyGroups(batchSize)
			// process users
			processDirtyUsers(batchSize)
		}
	}
}

// CleanStaleDirty removes entries from dirty zsets older than retention
func CleanStaleDirty(retention time.Duration) {
	cutoff := time.Now().Add(-retention).Unix()
	// 获取过期的 group ids（score <= cutoff）并尝试写回
	groupVals, err := rdb.Rdb.ZRangeByScore(groupsDirtyKey, gredis.ZRangeBy{Min: "-inf", Max: fmt.Sprintf("%d", cutoff)}).Result()
	if err != nil {
		log.Printf("flusher: fetch stale groups dirty error: %v", err)
	} else {
		for _, s := range groupVals {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				continue
			}
			// try write back once
			if err := writeBackGroup(id); err != nil {
				log.Printf("flusher: writeBackGroup failed for %d: %v", id, err)
				// keep the dirty mark for future attempts
				continue
			}
			// remove dirty mark after successful writeBack
			if err := rdb.Rdb.ZRem(groupsDirtyKey, s).Err(); err != nil {
				log.Printf("flusher: remove stale group dirty mark failed for %d: %v", id, err)
			}
		}
	}
	// 获取过期的 user ids 并尝试写回
	userVals, err := rdb.Rdb.ZRangeByScore(usersDirtyKey, gredis.ZRangeBy{Min: "-inf", Max: fmt.Sprintf("%d", cutoff)}).Result()
	if err != nil {
		log.Printf("flusher: fetch stale users dirty error: %v", err)
	} else {
		for _, s := range userVals {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				continue
			}
			if err := writeBackUser(id); err != nil {
				log.Printf("flusher: writeBackUser failed for %d: %v", id, err)
				continue
			}
			if err := rdb.Rdb.ZRem(usersDirtyKey, s).Err(); err != nil {
				log.Printf("flusher: remove stale user dirty mark failed for %d: %v", id, err)
			}
		}
	}
}

// writeBackGroup writes the current cached members of a group back to DB.
// If cache is missing, try to read DB to avoid destructive overwrite; return nil if nothing to change.
func writeBackGroup(roomID int64) error {
	members, err := GetRoomMemberIDsCache(roomID)
	if err != nil {
		// cache missing or error; try to read DB (nothing to write)
		log.Printf("flusher: cannot get members from cache for room %d: %v", roomID, err)
		return nil
	}
	// ensure table exists
	if err := CreateRoomMemberTable(roomID); err != nil {
		return err
	}
	tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
	tx, err := mysql.DB.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s", tableName)); err != nil {
		tx.Rollback()
		return err
	}
	if len(members) > 0 {
		query := fmt.Sprintf("INSERT INTO %s (user_id) VALUES ", tableName)
		vals := []interface{}{}
		parts := []string{}
		for _, u := range members {
			parts = append(parts, "(?)")
			vals = append(vals, u)
		}
		query = query + strings.Join(parts, ",")
		if _, err := tx.Exec(query, vals...); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// writeBackUser writes user's joined rooms from cache back to users table.
func writeBackUser(userID int64) error {
	rooms, err := GetUserJoinedRoomsCache(userID)
	if err != nil {
		log.Printf("flusher: cannot get joined rooms from cache for user %d: %v", userID, err)
		return nil
	}
	strs := []string{}
	for _, r := range rooms {
		strs = append(strs, fmt.Sprintf("%d", r))
	}
	csv := ""
	if len(strs) > 0 {
		csv = strings.Join(strs, ",") + ","
	}
	query := "UPDATE users SET joined_chatrooms = ? WHERE id = ?"
	if _, err := mysql.DB.Exec(query, csv, userID); err != nil {
		return err
	}
	return nil
}
func processDirtyGroups(batch int) {
	ids, err := PopDirtyGroups(int64(batch))
	if err != nil {
		log.Printf("flusher: pop dirty groups error: %v", err)
		return
	}
	for _, roomID := range ids {
		// get current members from redis
		members, err := GetRoomMemberIDsCache(roomID)
		if err != nil {
			log.Printf("flusher: get members from cache failed for room %d: %v", roomID, err)
			continue
		}
		// ensure table exists
		if err := CreateRoomMemberTable(roomID); err != nil {
			log.Printf("flusher: ensure table failed for room %d: %v", roomID, err)
			continue
		}
		// replace members in DB: simple approach -> delete all then insert
		tableName := fmt.Sprintf("chat_room_members_room_%d", roomID)
		tx, err := mysql.DB.Begin()
		if err != nil {
			log.Printf("flusher: begin tx failed: %v", err)
			continue
		}
		if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s", tableName)); err != nil {
			log.Printf("flusher: delete members failed for room %d: %v", roomID, err)
			tx.Rollback()
			continue
		}
		if len(members) > 0 {
			// prepare insert
			query := fmt.Sprintf("INSERT INTO %s (user_id) VALUES ", tableName)
			vals := []interface{}{}
			parts := []string{}
			for _, u := range members {
				parts = append(parts, "(?)")
				vals = append(vals, u)
			}
			query = query + strings.Join(parts, ",")
			if _, err := tx.Exec(query, vals...); err != nil {
				log.Printf("flusher: insert members failed for room %d: %v", roomID, err)
				tx.Rollback()
				continue
			}
		}
		if err := tx.Commit(); err != nil {
			log.Printf("flusher: commit failed for room %d: %v", roomID, err)
			continue
		}
		// remove dirty mark
		if err := RemoveDirtyGroup(roomID); err != nil {
			log.Printf("flusher: remove dirty mark failed for room %d: %v", roomID, err)
		}
	}
}

func processDirtyUsers(batch int) {
	ids, err := PopDirtyUsers(int64(batch))
	if err != nil {
		log.Printf("flusher: pop dirty users error: %v", err)
		return
	}
	for _, userID := range ids {
		rooms, err := GetUserJoinedRoomsCache(userID)
		if err != nil {
			log.Printf("flusher: get user joined rooms failed for user %d: %v", userID, err)
			continue
		}
		// convert to CSV like existing schema
		strs := []string{}
		for _, r := range rooms {
			strs = append(strs, fmt.Sprintf("%d", r))
		}
		csv := ""
		if len(strs) > 0 {
			csv = strings.Join(strs, ",") + ","
		}
		// update users table
		query := "UPDATE users SET joined_chatrooms = ? WHERE id = ?"
		if _, err := mysql.DB.Exec(query, csv, userID); err != nil {
			log.Printf("flusher: update joined_chatrooms failed for user %d: %v", userID, err)
			continue
		}
		if err := RemoveDirtyUser(userID); err != nil {
			log.Printf("flusher: remove dirty user mark failed for user %d: %v", userID, err)
		}
	}
}
