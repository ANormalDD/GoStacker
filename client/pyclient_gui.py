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
    print(f"Converting HTTP URL to WS URL: {http_url}")
    p = urlparse(http_url)
    scheme = "ws" if p.scheme == "http" else "wss"
    netloc = p.netloc
    print(f"Parsed URL - scheme: {scheme}, netloc: {netloc}")
    return f"{scheme}://{netloc}/api/ws"


class WorkerSignals(QtCore.QObject):
    message = QtCore.Signal(str)
    status = QtCore.Signal(str)


class PyClientGUI(QtWidgets.QWidget):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("GoStacker PySide6 Client")
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
        self.login_btn = QtWidgets.QPushButton("Login")
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
        self.connect_ws_btn = QtWidgets.QPushButton("Connect WebSocket")
        info_row.addWidget(self.connect_ws_btn)
        layout.addLayout(info_row)

        # Messages display
        self.msg_view = QtWidgets.QTextEdit()
        self.msg_view.setReadOnly(True)
        layout.addWidget(self.msg_view)

        # Send message controls
        send_row = QtWidgets.QHBoxLayout()
        send_row.addWidget(QtWidgets.QLabel("Room:"))
        self.room_combo = QtWidgets.QComboBox()
        self.room_combo.setEditable(False)
        send_row.addWidget(self.room_combo)
        self.refresh_rooms_btn = QtWidgets.QPushButton("Refresh Joined Groups")
        send_row.addWidget(self.refresh_rooms_btn)
        self.msg_input = QtWidgets.QLineEdit()
        self.msg_input.setPlaceholderText("Message text...")
        send_row.addWidget(self.msg_input)
        self.send_btn = QtWidgets.QPushButton("Send (HTTP)")
        send_row.addWidget(self.send_btn)
        layout.addLayout(send_row)

        # Group actions
        group_row = QtWidgets.QHBoxLayout()
        self.search_btn = QtWidgets.QPushButton("Search Groups")
        self.request_join_btn = QtWidgets.QPushButton("Request to Join Group")
        self.list_requests_btn = QtWidgets.QPushButton("Query Join Requests")
        group_row.addWidget(self.search_btn)
        group_row.addWidget(self.request_join_btn)
        group_row.addWidget(self.list_requests_btn)
        layout.addLayout(group_row)

        # Status bar
        self.status = QtWidgets.QLabel("")
        layout.addWidget(self.status)

    def _connect_signals(self):
        self.login_btn.clicked.connect(self.do_login)
        self.connect_ws_btn.clicked.connect(self.toggle_ws)
        self.send_btn.clicked.connect(self.do_send_message)
        self.refresh_rooms_btn.clicked.connect(self.fetch_joined_rooms)
        self.search_btn.clicked.connect(self.do_search)
        self.request_join_btn.clicked.connect(self.do_request_join)
        self.list_requests_btn.clicked.connect(self.do_list_requests)
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
            self.set_status("Please fill in meta backend URL first")
            return
        payload = {"username": self.username.text().strip(), "password": self.password.text()}
        try:
            r = requests.post(f"{meta_base}/login", json=payload, timeout=5)
            if r.status_code != 200:
                self.set_status(f"Login failed: {r.status_code}")
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
                self.set_status("Login successful")
                # Auto refresh joined groups list
                QtCore.QTimer.singleShot(100, self.fetch_joined_rooms)
            else:
                self.set_status("Login did not return token")
        except Exception as e:
            self.set_status(f"Login error: {e}")

    def _determine_ws_url(self):
        send_base = self.send_backend_edit.text().strip()
        # Get registry URL (assume same host as send, port 8084)
        p = urlparse(send_base)
        registry_url = f"{p.scheme}://{p.hostname}:8083"
        
        headers_req = {"Authorization": f"Bearer {self.token}"} if self.token else {}
        try:
            r = requests.get(f"{registry_url}/registry/gateway/available", headers=headers_req, timeout=5)
            #show result
            self.append_message(f"Registry response: {r.text}")
            if r.status_code == 200:
                obj = r.json()
                data = obj.get("data", {})
                addr = data.get("address")
                port = data.get("port")
                print(f"Discovered gateway at {addr}:{port}")
                if addr and port:
                    gateway_base = f"http://{addr}:{port}"
                    print(f"Using gateway base URL: {gateway_base}")
                elif addr:
                    if addr.startswith("http://") or addr.startswith("https://"):
                        gateway_base = addr
                    else:
                        gateway_base = f"http://{addr}"
                    return to_ws_url(gateway_base)
        except Exception:
            pass
        # fallback
        return to_ws_url(gateway_base)

    def toggle_ws(self):
        if self.ws_app:
            self.set_status("Closing WS...")
            try:
                self.ws_app.close()
            except Exception:
                pass
            self.ws_app = None
            self.set_status("WS disconnected")
            return

        if not self.token:
            self.set_status("Please login to get token first")
            return

        ws_url = self._determine_ws_url()
        print(f"Connecting to WS URL: {ws_url}")
        self.set_status(f"Connecting to {ws_url} ...")

        headers = [f"Authorization: Bearer {self.token}"]

        def on_message(ws, message):
            try:
                obj = json.loads(message)
                txt = json.dumps(obj, ensure_ascii=False)
            except Exception:
                txt = message
            self.signals.message.emit(txt)

        def on_error(ws, error):
            self.signals.status.emit(f"WS Error: {error}")

        def on_close(ws, code, reason):
            self.signals.status.emit(f"WS Closed: {code} {reason}")

        def on_open(ws):
            self.signals.status.emit(f"WS Connected: {ws_url}")

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
                self.signals.status.emit(f"WS Run Error: {e}")

        self.ws_thread = threading.Thread(target=run_ws, daemon=True)
        self.ws_thread.start()

    def do_send_message(self):
        if not self.token:
            self.set_status("Please login first")
            return
        send_base = self.send_backend_edit.text().strip()
        if self.room_combo.count() == 0:
            self.set_status("No joined groups, please refresh or join a group first")
            return
        room_text = self.room_combo.currentText()
        try:
            room = int(room_text)
        except Exception:
            self.set_status("Invalid room ID")
            return
        text = self.msg_input.text().strip()
        if not text:
            self.set_status("Message is empty")
            return
        payload = {"room_id": room, "content": {"type": "text", "text": text}}
        headers = {"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"}
        
        # Get Send instance from Registry
        actual_send_base = send_base
        p = urlparse(send_base)
        registry_url = f"{p.scheme}://{p.hostname}:8084"
        try:
            r = requests.get(f"{registry_url}/registry/send/available", headers=headers, timeout=5)
            if r.status_code == 200:
                obj = r.json()
                data = obj.get("data", {})
                send_url = data.get("url")
                if send_url:
                    actual_send_base = send_url
                    self.set_status(f"Using Send instance: {actual_send_base}")
        except Exception as e:
            self.set_status(f"Registry unavailable, using default Send: {e}")
        
        try:
            r = requests.post(f"{actual_send_base}/api/chat/send_message", json=payload, headers=headers, timeout=5)
            if r.status_code == 200:
                self.set_status("Send successful")
            else:
                self.set_status(f"Send failed: {r.status_code}")
                try:
                    self.append_message(r.text)
                except Exception:
                    pass
        except Exception as e:
            self.set_status(f"Send error: {e}")

    def fetch_joined_rooms(self):
        if not self.token:
            self.set_status("Please login first")
            return
        meta_base = self.meta_backend_edit.text().strip()
        headers = {"Authorization": f"Bearer {self.token}"}
        try:
            r = requests.get(f"{meta_base}/api/joined_rooms", headers=headers, timeout=5)
            if r.status_code != 200:
                self.set_status(f"Failed to refresh joined groups: {r.status_code}")
                try:
                    self.append_message(r.text)
                except Exception:
                    pass
                return
            obj = r.json()
            data = obj.get("data", {})
            room_ids = data.get("room_ids", [])
            self.room_combo.clear()
            for rid in room_ids:
                # show id as string
                try:
                    self.room_combo.addItem(str(rid))
                except Exception:
                    pass
            self.set_status(f"Loaded {len(room_ids)} joined groups")
        except Exception as e:
            self.set_status(f"Error getting joined groups: {e}")

    def do_search(self):
        if not self.token:
            self.set_status("Please login first")
            return
        q, ok = QtWidgets.QInputDialog.getText(self, "Search Groups", "Search Keyword:")
        if not ok:
            return
        limit, ok = QtWidgets.QInputDialog.getInt(self, "Search Groups", "limit:", 20, 1, 1000)
        if not ok:
            return
        headers = {"Authorization": f"Bearer {self.token}"}
        try:
            r = requests.get(f"{self.meta_backend_edit.text().strip()}/api/chat/group/search", params={"q": q, "limit": limit}, headers=headers, timeout=5)
            if r.status_code == 200:
                self.append_message(json.dumps(r.json(), ensure_ascii=False))
            else:
                self.append_message(f"search failed: {r.status_code} {r.text}")
        except Exception as e:
            self.set_status(f"search error: {e}")

    def do_request_join(self):
        if not self.token:
            self.set_status("Please login first")
            return
        room_id, ok = QtWidgets.QInputDialog.getInt(self, "Request to Join Group", "Room ID:")
        if not ok:
            return
        msg, ok = QtWidgets.QInputDialog.getText(self, "Request to Join Group", "Message (optional):")
        if not ok:
            msg = ""
        headers = {"Authorization": f"Bearer {self.token}", "Content-Type": "application/json"}
        try:
            r = requests.post(f"{self.meta_backend_edit.text().strip()}/api/chat/group/join/request", json={"room_id": room_id, "message": msg}, headers=headers, timeout=5)
            if r.status_code == 200:
                self.append_message("request created: " + json.dumps(r.json(), ensure_ascii=False))
            else:
                self.append_message(f"request failed: {r.status_code} {r.text}")
        except Exception as e:
            self.set_status(f"request error: {e}")

    def do_list_requests(self):
        if not self.token:
            self.set_status("Please login first")
            return
        room_id, ok = QtWidgets.QInputDialog.getInt(self, "Query Join Requests", "Room ID:")
        if not ok:
            return
        headers = {"Authorization": f"Bearer {self.token}"}
        try:
            r = requests.get(f"{self.meta_backend_edit.text().strip()}/api/chat/group/join/requests", params={"room_id": room_id}, headers=headers, timeout=5)
            if r.status_code == 200:
                self.append_message(json.dumps(r.json(), ensure_ascii=False))
            else:
                self.append_message(f"list failed: {r.status_code} {r.text}")
        except Exception as e:
            self.set_status(f"list error: {e}")

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
