#!/usr/bin/env python3
import argparse
import json
import os
import threading
import sys
import time
from urllib.parse import urlparse

import requests
import websocket

def to_ws_url(http_url: str) -> str:
    p = urlparse(http_url)
    scheme = "ws" if p.scheme == "http" else "wss"
    netloc = p.netloc
    return f"{scheme}://{netloc}/api/ws"

def register(meta_base):
    username = input("username: ")
    password = input("password: ")
    nickname = input("nickname (optional): ")
    payload = {"username": username, "password": password, "nickname": nickname}
    r = requests.post(f"{meta_base}/register", json=payload)
    print(r.status_code, r.text)
    return r.status_code == 200


def login(meta_base):
    username = input("username: ")
    password = input("password: ")
    payload = {"username": username, "password": password}
    r = requests.post(f"{meta_base}/login", json=payload)
    print(r.status_code)
    try:
        obj = r.json()
        print(json.dumps(obj, indent=2, ensure_ascii=False))
        data = obj.get("data", {})
        if "token" in data:
            print("登录成功！")
            return data["token"]
    except Exception:
        print(r.text)
    return None


def create_room(meta_base, token):
    if not token:
        print("请先登录！")
        return
    name = input("room name: ")
    members = input("member ids (comma separated, optional): ")
    member_ids = []
    if members.strip():
        for s in members.split(','):
            try:
                member_ids.append(int(s.strip()))
            except Exception:
                pass
    payload = {"name": name, "is_group": True, "member_ids": member_ids}
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.post(f"{meta_base}/api/chat/group/create", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)

def create_private_room(meta_base, token):
    if not token:
        print("请先登录！")
        return
    name = input("room name: ")
    member_id = input("member id: ")
    member_ids = []
    try:
        member_ids.append(int(member_id.strip()))
    except Exception:
        pass
    payload = {"name": name, "is_group": False, "member_ids": member_ids}
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.post(f"{meta_base}/api/chat/group/create", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)
        return
    
def ws_connect(send_base, token, registry_url=None):
    if not token:
        print("请先登录！")
        return
    # Get gateway address from Registry service
    # If registry_url is not provided, try to infer from send_base (assume port 8084)
    if not registry_url:
        # Default: assume registry is on same host as send, port 8084
        p = urlparse(send_base)
        registry_url = f"{p.scheme}://{p.hostname}:8084"
    
    headers_req = {"Authorization": f"Bearer {token}"}
    try:
        r = requests.get(f"{registry_url}/registry/gateway/available", headers=headers_req, timeout=5)
        print("Registry response status:", r.status_code)
        print("Registry response text:", r.text)
        if r.status_code == 200:
            try:
                obj = r.json()
                data = obj.get("data", {})
                addr = data.get("address")
                port = data.get("port")
                if addr and port:
                    # Build gateway URL from address and port
                    gateway_base = f"http://{addr}:{port}"
                elif addr:
                    # Fallback: if port is missing, use address as-is
                    if addr.startswith("http://") or addr.startswith("https://"):
                        gateway_base = addr
                    else:
                        gateway_base = f"http://{addr}"
                    ws_url = to_ws_url(gateway_base)
                else:
                    ws_url = to_ws_url(send_base)
            except Exception:
                ws_url = to_ws_url(send_base)
        else:
            return
    except Exception:
        ws_url = to_ws_url(send_base)
    
    headers = [f"Authorization: Bearer {token}"]
    # pretty print incoming JSON messages when possible
    def on_message(ws, message):
        try:
            obj = json.loads(message)
            print("<", json.dumps(obj, ensure_ascii=False, indent=2))
        except Exception:
            print("<", message)
    def on_error(ws, error):
        print("error:", error)
    def on_close(ws, close_status_code, close_msg):
        print("closed", close_status_code, close_msg)
    def on_open(ws):
        print("connected to", ws_url)
    # optional: log ping/pong events from server for visibility
    def on_ping(ws, message):
        try:
            print("<< ping from server:", message)
        except Exception:
            print("<< ping from server")
    def on_pong(ws, message):
        try:
            print("<< pong from server:", message)
        except Exception:
            print("<< pong from server")
    ws = websocket.WebSocketApp(ws_url,
                                header=headers,
                                on_message=on_message,
                                on_error=on_error,
                                on_close=on_close,
                                on_open=on_open,
                                on_ping=on_ping,
                                on_pong=on_pong)
    wst = threading.Thread(target=ws.run_forever, kwargs={"sslopt": {"cert_reqs": 0}})
    # run in background thread and allow automatic reconnection logic in outer loop
    def start_ws():
        return threading.Thread(target=ws.run_forever, kwargs={"sslopt": {"cert_reqs": 0}})

    backoff = 1
    wst = start_ws()
    wst.daemon = True
    wst.start()

    try:
        while True:
            line = sys.stdin.readline()
            if not line:
                break
            line = line.strip()
            if line in ("quit", "exit"):
                ws.close()
                break
            if line == "/help":
                print("Commands:\n  /send <room_id> <text>   send text message to room\n  /quit or /exit            quit client\n  plain text (not recognized) will be sent raw to WS")
                continue
            if line.startswith("/send "):
                # /send <room_id> <text>
                parts = line.split(' ', 2)
                if len(parts) < 3:
                    print("usage: /send <room_id> <text>")
                    continue
                try:
                    room_id = int(parts[1])
                except Exception:
                    print("invalid room_id")
                    continue
                text = parts[2]
                payload = {"room_id": room_id, "content": {"type": "text", "text": text}}
                headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
                
                # Get Send instance from Registry
                actual_send_base = send_base
                try:
                    r = requests.get(f"{registry_url}/registry/send/available", headers=headers, timeout=5)
                    if r.status_code == 200:
                        obj = r.json()
                        data = obj.get("data", {})
                        send_url = data.get("url")
                        if send_url:
                            actual_send_base = send_url
                            print(f"Using Send instance: {actual_send_base}")
                except Exception as e:
                    print(f"Registry unavailable, using default Send: {e}")
                
                try:
                    r = requests.post(f"{actual_send_base}/api/chat/send_message", json=payload, headers=headers, timeout=5)
                    if r.status_code == 200:
                        print("> sent")
                    else:
                        print("> send failed", r.status_code, r.text)
                except Exception as e:
                    print("> send error:", e)
                continue
            if line:
                # try to send plain text over WS (server expects JSON from push, but raw is okay for debug)
                try:
                    ws.send(line)
                except Exception as e:
                    print("send error:", e)
                    # attempt to restart connection
                    try:
                        ws.close()
                    except Exception:
                        pass
                    print("attempting to reconnect in", backoff, "s")
                    time.sleep(backoff)
                    backoff = min(backoff * 2, 30)
                    ws.run_forever(sslopt={"cert_reqs": 0})
    except KeyboardInterrupt:
        ws.close()


def search_rooms(meta_base, token):
    q = input("search q: ").strip()
    limit = input("limit (default 20): ").strip() or "20"
    try:
        l = int(limit)
    except Exception:
        l = 20
    params = {"q": q, "limit": l}
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.get(f"{meta_base}/api/chat/group/search", params=params, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)


def join_group(meta_base, token):
    room_id = input("room id to join: ").strip()
    try:
        rid = int(room_id)
    except Exception:
        print("invalid room id")
        return
    payload = {"room_id": rid}
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.post(f"{meta_base}/api/chat/group/join", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)


def request_join(meta_base, token):
    room_id = input("room id to request join: ").strip()
    msg = input("optional message: ").strip()
    try:
        rid = int(room_id)
    except Exception:
        print("invalid room id")
        return
    payload = {"room_id": rid, "message": msg}
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.post(f"{meta_base}/api/chat/group/join/request", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)


def list_join_requests(meta_base, token):
    room_id = input("room id to list requests: ").strip()
    try:
        rid = int(room_id)
    except Exception:
        print("invalid room id")
        return
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.get(f"{meta_base}/api/chat/group/join/requests", params={"room_id": rid}, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)


def respond_join_request(meta_base, token):
    reqid = input("request id to respond: ").strip()
    approve = input("approve? (y/n): ").strip().lower() == 'y'
    try:
        rid = int(reqid)
    except Exception:
        print("invalid request id")
        return
    payload = {"request_id": rid, "approve": approve}
    headers = {"Authorization": f"Bearer {token}"}
    r = requests.post(f"{meta_base}/api/chat/group/join/respond", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)

def main_loop(send_base, meta_base):
    token = None
    while True:
        print("\n==== GoStacker 客户端 ====")
        if not token:
            print("1. 注册")
            print("2. 登录")
            print("0. 退出")
            choice = input("请选择: ").strip()
            if choice == "1":
                register(meta_base)
            elif choice == "2":
                t = login(meta_base)
                if t:
                    token = t
            elif choice == "0":
                print("再见！")
                break
            else:
                print("无效选择")
        else:
                print("1. 创建聊天室")
                print("2. 连接 WebSocket (聊天/收消息)")
                print("3. 注销登录")
                print("4. 创建私聊")
                print("5. 搜索群")
                print("6. 主动加入群")
                print("7. 提交入群申请")
                print("8. 列出入群申请(管理员/群主)")
                print("9. 审核入群申请(管理员/群主)")
                print("0. 退出")
                choice = input("请选择: ").strip()
                if choice == "1":
                    create_room(meta_base, token)
                elif choice == "2":
                    ws_connect(send_base, token)
                elif choice == "3":
                    token = None
                    print("已注销登录")
                elif choice == "4":
                    create_private_room(meta_base, token)
                elif choice == "5":
                    search_rooms(meta_base, token)
                elif choice == "6":
                    join_group(meta_base, token)
                elif choice == "7":
                    request_join(meta_base, token)
                elif choice == "8":
                    list_join_requests(meta_base, token)
                elif choice == "9":
                    respond_join_request(meta_base, token)
                elif choice == "0":
                    print("再见！")
                    break
                else:
                    print("无效选择")


def main():
    import argparse
    parser = argparse.ArgumentParser(description="GoStacker 交互式 CLI 客户端 (python)")
    parser.add_argument("--send-backend", dest="send_backend", default="http://localhost:8081", help="send backend base url (default: http://localhost:8081)")
    parser.add_argument("--meta-backend", dest="meta_backend", default="http://localhost:8082", help="meta backend base url (default: http://localhost:8082)")
    args = parser.parse_args()
    main_loop(args.send_backend, args.meta_backend)


if __name__ == "__main__":
    main()
