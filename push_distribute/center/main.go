package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/go-redis/redis"
)

var rdb *redis.Client

func main() {
	rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if _, err := rdb.Ping().Result(); err != nil {
		log.Fatal("Failed to connect redis: ", err)
	}

	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/unregister", unregisterHandler)
	http.HandleFunc("/query", queryHandler)
	http.HandleFunc("/report", reportHandler)
	http.HandleFunc("/select_gateway", selectGatewayHandler)
	http.HandleFunc("/handoff", handoffHandler)
	// forward endpoint: accept forwarded push tasks from other gateway
	http.HandleFunc("/forward", forwardHandler)

	fmt.Println("Central server listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", nil))
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	b, _ := ioutil.ReadAll(r.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		w.WriteHeader(400)
		return
	}
	uidf, ok := data["user_id"]
	if !ok {
		w.WriteHeader(400)
		return
	}
	uid := int64(uidf.(float64))
	gid, _ := data["gateway_id"].(string)
	addr, _ := data["gateway_addr"].(string)
	if gid == "" {
		gid = addr
	}
	// set mapping user->gateway
	if err := rdb.Set(fmt.Sprintf("user:gateway:%d", uid), gid, 0).Err(); err != nil {
		w.WriteHeader(500)
		return
	}
	// add to gateway set
	if err := rdb.SAdd(fmt.Sprintf("gateway:users:%s", gid), strconv.FormatInt(uid, 10)).Err(); err != nil {
		w.WriteHeader(500)
		return
	}
	// store gateway addr
	if addr != "" {
		if err := rdb.Set(fmt.Sprintf("gateway:addr:%s", gid), addr, 0).Err(); err != nil {
			w.WriteHeader(500)
			return
		}
	}
	w.WriteHeader(200)
}

func unregisterHandler(w http.ResponseWriter, r *http.Request) {
	b, _ := ioutil.ReadAll(r.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		w.WriteHeader(400)
		return
	}
	uidf, ok := data["user_id"]
	if !ok {
		w.WriteHeader(400)
		return
	}
	uid := int64(uidf.(float64))
	gid, _ := data["gateway_id"].(string)
	// remove mapping
	if err := rdb.Del(fmt.Sprintf("user:gateway:%d", uid)).Err(); err != nil {
		// log but continue
	}
	if gid != "" {
		rdb.SRem(fmt.Sprintf("gateway:users:%s", gid), strconv.FormatInt(uid, 10))
	}
	w.WriteHeader(200)
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	uidStr := q.Get("user_id")
	if uidStr == "" {
		w.WriteHeader(400)
		return
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	gid, err := rdb.Get(fmt.Sprintf("user:gateway:%d", uid)).Result()
	if err == redis.Nil || gid == "" {
		w.WriteHeader(404)
		return
	}
	addr, _ := rdb.Get(fmt.Sprintf("gateway:addr:%s", gid)).Result()
	resp := map[string]string{"gateway_id": gid, "gateway_addr": addr}
	b, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func forwardHandler(w http.ResponseWriter, r *http.Request) {
	// For now center does not forward; other gateways should post to the target gateway directly
	w.WriteHeader(200)
}

func reportHandler(w http.ResponseWriter, r *http.Request) {
	b, _ := ioutil.ReadAll(r.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		w.WriteHeader(400)
		return
	}
	gid, _ := data["gateway_id"].(string)
	if gid == "" {
		w.WriteHeader(400)
		return
	}
	// store raw JSON under gateway:load:<gid>
	if err := rdb.Set(fmt.Sprintf("gateway:load:%s", gid), string(b), 0).Err(); err != nil {
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
}

func selectGatewayHandler(w http.ResponseWriter, r *http.Request) {
	// find all gateways by scanning keys gateway:addr:*
	keys, err := rdb.Keys("gateway:addr:*").Result()
	if err != nil {
		w.WriteHeader(500)
		return
	}
	var bestGid, bestAddr string
	bestUtil := 1.1 // utilization ratio, lower is better
	for _, k := range keys {
		// k is gateway:addr:<gid>
		gid := k[len("gateway:addr:"):]
		addr, err := rdb.Get(k).Result()
		if err != nil {
			continue
		}
		loadStr, err := rdb.Get(fmt.Sprintf("gateway:load:%s", gid)).Result()
		if err != nil {
			// unknown load -> treat as medium (0.5)
			if bestUtil > 0.5 {
				bestUtil = 0.5
				bestGid = gid
				bestAddr = addr
			}
			continue
		}
		var load map[string]interface{}
		if err := json.Unmarshal([]byte(loadStr), &load); err != nil {
			continue
		}
		qlenF, _ := load["queue_len"].(float64)
		qcapF, _ := load["queue_cap"].(float64)
		if qcapF <= 0 {
			continue
		}
		util := qlenF / qcapF
		if util < bestUtil {
			bestUtil = util
			bestGid = gid
			bestAddr = addr
		}
	}
	if bestGid == "" {
		w.WriteHeader(404)
		return
	}
	resp := map[string]string{"gateway_id": bestGid, "gateway_addr": bestAddr}
	b, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func handoffHandler(w http.ResponseWriter, r *http.Request) {
	var reader io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		defer gz.Close()
		reader = gz
	}
	b, _ := ioutil.ReadAll(reader)
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		w.WriteHeader(400)
		return
	}
	tasksIface, ok := data["tasks"]
	if !ok {
		w.WriteHeader(400)
		return
	}
	tasks, ok := tasksIface.([]interface{})
	if !ok {
		w.WriteHeader(400)
		return
	}
	for _, ti := range tasks {
		tmap, ok := ti.(map[string]interface{})
		if !ok {
			continue
		}
		uidF, ok := tmap["user_id"].(float64)
		if !ok {
			continue
		}
		uid := int64(uidF)
		msgStr, _ := tmap["message"].(string)
		uidStr := strconv.FormatInt(uid, 10)
		// add to wait set and wait list
		if err := rdb.SAdd("wait:push:set", uidStr).Err(); err != nil {
			// log and continue
		}
		if err := rdb.RPush("wait:push:"+uidStr, msgStr).Err(); err != nil {
			// log and continue
		}
	}
	w.WriteHeader(200)
}
