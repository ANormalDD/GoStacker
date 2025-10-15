#!/usr/bin/env python3
import argparse
import json
import os
import threading
import sys
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
    r = requests.post(f"{base}/api/chat/create", json=payload, headers=headers)
    print(r.status_code)
    try:
        print(json.dumps(r.json(), indent=2, ensure_ascii=False))
    except Exception:
        print(r.text)


def ws_connect(base, token):
    if not token:
        print("请先登录！")
        return
    ws_url = to_ws_url(base)
    headers = [f"Authorization: Bearer {token}"]
    def on_message(ws, message):
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
            if line:
                ws.send(line)
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
