#!/usr/bin/env python3
"""A simple PySide6 GUI client for GoStacker.

Features:
- Enter backend URL
- Login to obtain token
- Connect to gateway WebSocket and display incoming messages
- Send message to a room via HTTP API

Dependencies: PySide6, requests, websocket-client
"""
import json
import threading
import time
from urllib.parse import urlparse

import requests
import websocket
from PySide6 import QtCore, QtWidgets


def to_ws_url(http_url: str) -> str:
    p = urlparse(http_url)
    scheme = "ws" if p.scheme == "http" else "wss"
    netloc = p.netloc
    return f"{scheme}://{netloc}/api/ws"


class WorkerSignals(QtCore.QObject):
    message = QtCore.Signal(str)
    status = QtCore.Signal(str)


class PyClientGUI(QtWidgets.QWidget):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("GoStacker PySide6 客户端")
        self.resize(800, 600)

        self.token = None
        self.ws_app = None
        self.ws_thread = None
        self.signals = WorkerSignals()

        self._build_ui()
        self._connect_signals()

    def _build_ui(self):
        layout = QtWidgets.QVBoxLayout(self)

        # Backends (send/meta)
        row = QtWidgets.QHBoxLayout()
        row.addWidget(QtWidgets.QLabel("Send Backend:"))
        self.send_backend_edit = QtWidgets.QLineEdit("http://localhost:8081")
        row.addWidget(self.send_backend_edit)
        row.addSpacing(12)
        row.addWidget(QtWidgets.QLabel("Meta Backend:"))
        self.meta_backend_edit = QtWidgets.QLineEdit("http://localhost:8082")
        row.addWidget(self.meta_backend_edit)
        layout.addLayout(row)

        # Login section
        form = QtWidgets.QHBoxLayout()
        self.username = QtWidgets.QLineEdit()
        self.username.setPlaceholderText("username")
        self.password = QtWidgets.QLineEdit()
        self.password.setEchoMode(QtWidgets.QLineEdit.Password)
        self.password.setPlaceholderText("password")
        self.login_btn = QtWidgets.QPushButton("登录")
        form.addWidget(self.username)
        form.addWidget(self.password)
        form.addWidget(self.login_btn)
        layout.addLayout(form)

        # Token / status
        info_row = QtWidgets.QHBoxLayout()
        info_row.addWidget(QtWidgets.QLabel("Token:"))
        self.token_edit = QtWidgets.QLineEdit()
        self.token_edit.setReadOnly(True)
        info_row.addWidget(self.token_edit)
        self.connect_ws_btn = QtWidgets.QPushButton("连接 WebSocket")
        info_row.addWidget(self.connect_ws_btn)
        layout.addLayout(info_row)

        # Messages display
        self.msg_view = QtWidgets.QTextEdit()
        self.msg_view.setReadOnly(True)
        layout.addWidget(self.msg_view)

        # Send message controls
        send_row = QtWidgets.QHBoxLayout()
        send_row.addWidget(QtWidgets.QLabel("Room ID:"))
        self.room_id = QtWidgets.QSpinBox()
        self.room_id.setMinimum(1)
        self.room_id.setMaximum(10_000_000)
        send_row.addWidget(self.room_id)
        self.msg_input = QtWidgets.QLineEdit()
        self.msg_input.setPlaceholderText("消息文本...")
        send_row.addWidget(self.msg_input)
        self.send_btn = QtWidgets.QPushButton("发送 (HTTP)")
        send_row.addWidget(self.send_btn)
        layout.addLayout(send_row)

        # Status bar
        self.status = QtWidgets.QLabel("")
        layout.addWidget(self.status)

    def _connect_signals(self):
        self.login_btn.clicked.connect(self.do_login)
        self.connect_ws_btn.clicked.connect(self.toggle_ws)
        self.send_btn.clicked.connect(self.do_send_message)
        self.signals.message.connect(self.append_message)
        self.signals.status.connect(self.set_status)

    def append_message(self, text: str):
        ts = time.strftime("%H:%M:%S")
        self.msg_view.append(f"[{ts}] {text}")

    def set_status(self, text: str):
        self.status.setText(text)

    def do_login(self):
        meta_base = self.meta_backend_edit.text().strip()
        if not meta_base:
            self.set_status("请先填写 meta backend URL")
            return
        payload = {"username": self.username.text().strip(), "password": self.password.text()}
        try:
            r = requests.post(f"{meta_base}/login", json=payload, timeout=5)
            if r.status_code != 200:
                self.set_status(f"登录失败: {r.status_code}")
                try:
                    self.append_message(r.text)
                except Exception:
                    pass
                return
            obj = r.json()
            data = obj.get("data", {})
            token = data.get("token")
            if token:
                self.token = token
                self.token_edit.setText(token)
                self.set_status("登录成功")
            else:
                self.set_status("登录没有返回 token")
        except Exception as e:
            self.set_status(f"登录错误: {e}")

    def _determine_ws_url(self):
        send_base = self.send_backend_edit.text().strip()
        headers_req = {"Authorization": f"Bearer {self.token}"} if self.token else {}
        try:
            r = requests.get(f"{send_base}/api/get_gateway_ws", headers=headers_req, timeout=5)
            if r.status_code == 200:
                obj = r.json()
                data = obj.get("data", {})
                addr = data.get("address")
                if addr:
                    if addr.startswith("http://") or addr.startswith("https://"):
                        gateway_base = addr
                    else:
                        gateway_base = f"http://{addr}"
                    return to_ws_url(gateway_base)
        except Exception:
            pass
        # fallback
        return to_ws_url(send_base)

    def toggle_ws(self):
        if self.ws_app:
            self.set_status("关闭 WS...")
            try:
                self.ws_app.close()
            except Exception:
                pass
            self.ws_app = None
            self.set_status("WS 已断开")
            return

        if not self.token:
            self.set_status("请先登录以获取 token")
            return

        ws_url = self._determine_ws_url()
        self.set_status(f"连接到 {ws_url} ...")

        headers = [f"Authorization: Bearer {self.token}"]

        def on_message(ws, message):
            try:
                obj = json.loads(message)
                txt = json.dumps(obj, ensure_ascii=False)
            except Exception:
                txt = message
            self.signals.message.emit(txt)

        def on_error(ws, error):
            self.signals.status.emit(f"WS 错误: {error}")

        def on_close(ws, code, reason):
            self.signals.status.emit(f"WS 关闭: {code} {reason}")

        def on_open(ws):
            self.signals.status.emit(f"WS 已连接: {ws_url}")

        self.ws_app = websocket.WebSocketApp(
            ws_url,
            header=headers,
            on_message=on_message,
            on_error=on_error,
            on_close=on_close,
            on_open=on_open,
        )

        def run_ws():
            try:
                self.ws_app.run_forever(sslopt={"cert_reqs": 0})
            except Exception as e:
                self.signals.status.emit(f"WS 运行错误: {e}")

        self.ws_thread = threading.Thread(target=run_ws, daemon=True)
        self.ws_thread.start()

    def do_send_message(self):
        if not self.token:
            self.set_status("请先登录")
            return
        send_base = self.send_backend_edit.text().strip()
        room = int(self.room_id.value())
        text = self.msg_input.text().strip()
        if not text:
            self.set_status("消息为空")
            return
        payload = {"room_id": room, "content": {"type": "text", "text": text}}
        headers = {"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"}
        try:
            r = requests.post(f"{send_base}/api/chat/send_message", json=payload, headers=headers, timeout=5)
            if r.status_code == 200:
                self.set_status("发送成功")
            else:
                self.set_status(f"发送失败: {r.status_code}")
                try:
                    self.append_message(r.text)
                except Exception:
                    pass
        except Exception as e:
            self.set_status(f"发送错误: {e}")

    def closeEvent(self, event: QtCore.QEvent) -> None:
        try:
            if self.ws_app:
                self.ws_app.close()
        except Exception:
            pass
        event.accept()


def main():
    import sys

    app = QtWidgets.QApplication(sys.argv)
    w = PyClientGUI()
    w.show()
    sys.exit(app.exec())


if __name__ == "__main__":
    main()
