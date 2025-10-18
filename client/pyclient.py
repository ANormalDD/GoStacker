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

def register(base):
    username = input("username: ")
    password = input("password: ")
    nickname = input("nickname (optional): ")
    payload = {"username": username, "password": password, "nickname": nickname}
    r = requests.post(f"{base}/register", json=payload)
    print(r.status_code, r.text)
    return r.status_code == 200


def login(base):
    username = input("username: ")
    password = input("password: ")
    payload = {"username": username, "password": password}
    r = requests.post(f"{base}/login", json=payload)
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


def create_room(base, token):
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
    r = requests.post(f"{base}/api/chat/group/create", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)

def create_private_room(base, token):
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
    r = requests.post(f"{base}/api/chat/group/create", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)
        return
    
def ws_connect(base, token):
    if not token:
        print("请先登录！")
        return
    ws_url = to_ws_url(base)
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
    ws = websocket.WebSocketApp(ws_url,
                                header=headers,
                                on_message=on_message,
                                on_error=on_error,
                                on_close=on_close,
                                on_open=on_open)
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
                try:
                    r = requests.post(f"{base}/api/chat/send_message", json=payload, headers=headers, timeout=5)
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

def main_loop(base):
    token = None
    while True:
        print("\n==== GoStacker 客户端 ====")
        if not token:
            print("1. 注册")
            print("2. 登录")
            print("0. 退出")
            choice = input("请选择: ").strip()
            if choice == "1":
                register(base)
            elif choice == "2":
                t = login(base)
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
            print("0. 退出")
            choice = input("请选择: ").strip()
            if choice == "1":
                create_room(base, token)
            elif choice == "2":
                ws_connect(base, token)
            elif choice == "3":
                token = None
                print("已注销登录")
            elif choice == "0":
                print("再见！")
                break
            elif choice == "4":
                create_private_room(base, token)
            else:
                print("无效选择")


def main():
    import argparse
    parser = argparse.ArgumentParser(description="GoStacker 交互式 CLI 客户端 (python)")
    parser.add_argument("--backend", default="http://localhost:8081", help="backend base url")
    args = parser.parse_args()
    main_loop(args.backend)


if __name__ == "__main__":
    main()
