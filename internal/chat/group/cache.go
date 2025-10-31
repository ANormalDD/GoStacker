package group

import (
	rdb "GoStacker/pkg/db/redis"
	"fmt"
	"strconv"
	"time"

	cfg "GoStacker/pkg/config"

	gredis "github.com/go-redis/redis"
)

const (
	groupMembersKeyFmt = "groups:members:%d"
	userJoinedKeyFmt   = "users:joined:%d"
	groupsDirtyKey     = "groups:dirty"
	usersDirtyKey      = "users:dirty"
	redisRetry         = 3
	// 默认缓存 TTL，读写时会续期。可改为可配置项。
	defaultCacheTTL = 24 * time.Hour
)

func groupMembersKey(roomID int64) string {
	return fmt.Sprintf(groupMembersKeyFmt, roomID)
}
func userJoinedKey(userID int64) string {
	return fmt.Sprintf(userJoinedKeyFmt, userID)
}

func getCacheTTL() time.Duration {
	if cfg.Conf != nil && cfg.Conf.GroupCacheConfig != nil {
		ttl := time.Duration(cfg.Conf.GroupCacheConfig.CacheTTLSeconds) * time.Second
		if ttl > 0 {
			return ttl
		}
	}
	return defaultCacheTTL
}

func AddRoomMemberCache(roomID int64, userID int64) error {
	// add to group members set and user's joined set, mark dirty
	gkey := groupMembersKey(roomID)
	ukey := userJoinedKey(userID)
	memberStr := strconv.FormatInt(userID, 10)
	roomStr := strconv.FormatInt(roomID, 10)

	if err := rdb.SAddWithRetry(redisRetry, gkey, memberStr); err != nil {
		return err
	}
	if err := rdb.SAddWithRetry(redisRetry, ukey, roomStr); err != nil {
		return err
	}
	// mark dirty
	score := float64(time.Now().Unix())
	if err := rdb.Rdb.ZAdd(groupsDirtyKey, gredis.Z{Score: score, Member: strconv.FormatInt(roomID, 10)}).Err(); err != nil {
		return err
	}
	if err := rdb.Rdb.ZAdd(usersDirtyKey, gredis.Z{Score: score, Member: strconv.FormatInt(userID, 10)}).Err(); err != nil {
		return err
	}
	// 设置/续期缓存 TTL
	// 标脏时保护 key 不被过期（持久化），以确保后台有机会把脏数据写回 DB
	_ = rdb.Rdb.Persist(gkey).Err()
	_ = rdb.Rdb.Persist(ukey).Err()
	return nil
}

func AddRoomMembersCache(roomID int64, userIDs []int64) error {
	gkey := groupMembersKey(roomID)
	members := make([]interface{}, 0, len(userIDs))
	roomStr := strconv.FormatInt(roomID, 10)
	for _, u := range userIDs {
		members = append(members, strconv.FormatInt(u, 10))
	}
	if err := rdb.Rdb.SAdd(gkey, members...).Err(); err != nil {
		// fallback to retry helper
		if err := rdb.SAddWithRetry(redisRetry, gkey, members...); err != nil {
			return err
		}
	}
	// update users' joined sets
	score := float64(time.Now().Unix())
	for _, u := range userIDs {
		ukey := userJoinedKey(u)
		if err := rdb.SAddWithRetry(redisRetry, ukey, roomStr); err != nil {
			return err
		}
		if err := rdb.Rdb.ZAdd(usersDirtyKey, gredis.Z{Score: score, Member: strconv.FormatInt(u, 10)}).Err(); err != nil {
			return err
		}
	}
	// mark group dirty
	if err := rdb.Rdb.ZAdd(groupsDirtyKey, gredis.Z{Score: score, Member: roomStr}).Err(); err != nil {
		return err
	}
	// 设置/续期缓存 TTL
	// 标脏时保护 key 不被过期（持久化）
	_ = rdb.Rdb.Persist(gkey).Err()
	for _, u := range userIDs {
		_ = rdb.Rdb.Persist(fmt.Sprintf(userJoinedKeyFmt, u)).Err()
	}
	return nil
}

func RemoveRoomMemberCache(roomID int64, userID int64) error {
	gkey := groupMembersKey(roomID)
	ukey := userJoinedKey(userID)
	memberStr := strconv.FormatInt(userID, 10)
	roomStr := strconv.FormatInt(roomID, 10)
	if err := rdb.Rdb.SRem(gkey, memberStr).Err(); err != nil && err != gredis.Nil {
		return err
	}
	if err := rdb.Rdb.SRem(ukey, roomStr).Err(); err != nil && err != gredis.Nil {
		return err
	}
	score := float64(time.Now().Unix())
	if err := rdb.Rdb.ZAdd(groupsDirtyKey, gredis.Z{Score: score, Member: roomStr}).Err(); err != nil {
		return err
	}
	if err := rdb.Rdb.ZAdd(usersDirtyKey, gredis.Z{Score: score, Member: strconv.FormatInt(userID, 10)}).Err(); err != nil {
		return err
	}
	_ = rdb.Rdb.Expire(gkey, getCacheTTL()).Err()
	_ = rdb.Rdb.Persist(gkey).Err()
	return nil
}

func GetRoomMemberIDsCache(roomID int64) ([]int64, error) {
	gkey := groupMembersKey(roomID)
	vals, err := rdb.SMembersWithRetry(redisRetry, gkey)
	if err != nil {
		return nil, err
	}
	// 访问时续期
	// 如果这个 group 当前在 dirty 集合中，保持持久化，防止被过期清理；否则做滑动 TTL
	roomStr := strconv.FormatInt(roomID, 10)
	if score, err := rdb.Rdb.ZScore(groupsDirtyKey, roomStr).Result(); err == nil && score > 0 {
		_ = rdb.Rdb.Persist(gkey).Err()
	} else {
		_ = rdb.Rdb.Expire(gkey, getCacheTTL()).Err()
	}
	res := make([]int64, 0, len(vals))
	for _, s := range vals {
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		res = append(res, id)
	}
	return res, nil
}

func IsRoomMemberCache(roomID int64, userID int64) (bool, error) {
	gkey := groupMembersKey(roomID)
	memberStr := strconv.FormatInt(userID, 10)
	exists, err := rdb.Rdb.SIsMember(gkey, memberStr).Result()
	if err != nil {
		return false, err
	}
	// 访问时续期
	// 访问时，如果该 key 在 dirty 集合中则保持持久化，否则续期
	roomStr := strconv.FormatInt(roomID, 10)
	if score, err := rdb.Rdb.ZScore(groupsDirtyKey, roomStr).Result(); err == nil && score > 0 {
		_ = rdb.Rdb.Persist(gkey).Err()
	} else {
		_ = rdb.Rdb.Expire(gkey, getCacheTTL()).Err()
	}
	return exists, nil
}

// PopDirtyGroups returns up to n dirty roomIDs (as int64). It does NOT remove them; caller decides when to remove.
func PopDirtyGroups(n int64) ([]int64, error) {
	// take smallest scores (oldest)
	vals, err := rdb.Rdb.ZRange(groupsDirtyKey, 0, n-1).Result()
	if err != nil {
		return nil, err
	}
	res := make([]int64, 0, len(vals))
	for _, s := range vals {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		res = append(res, id)
	}
	return res, nil
}

func RemoveDirtyGroup(roomID int64) error {
	return rdb.Rdb.ZRem(groupsDirtyKey, strconv.FormatInt(roomID, 10)).Err()
}

func PopDirtyUsers(n int64) ([]int64, error) {
	vals, err := rdb.Rdb.ZRange(usersDirtyKey, 0, n-1).Result()
	if err != nil {
		return nil, err
	}
	res := make([]int64, 0, len(vals))
	for _, s := range vals {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		res = append(res, id)
	}
	return res, nil
}

func RemoveDirtyUser(userID int64) error {
	return rdb.Rdb.ZRem(usersDirtyKey, strconv.FormatInt(userID, 10)).Err()
}

func GetUserJoinedRoomsCache(userID int64) ([]int64, error) {
	ukey := userJoinedKey(userID)
	vals, err := rdb.SMembersWithRetry(redisRetry, ukey)
	if err != nil {
		return nil, err
	}
	// 访问时续期
	// 访问时，如果该 key 在 dirty 集合中则保持持久化，否则续期
	userStr := strconv.FormatInt(userID, 10)
	if score, err := rdb.Rdb.ZScore(usersDirtyKey, userStr).Result(); err == nil && score > 0 {
		_ = rdb.Rdb.Persist(ukey).Err()
	} else {
		_ = rdb.Rdb.Expire(ukey, defaultCacheTTL).Err()
	}
	res := make([]int64, 0, len(vals))
	for _, s := range vals {
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		res = append(res, id)
	}
	return res, nil
}
