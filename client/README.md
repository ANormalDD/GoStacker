GoStacker CLI client

This is a minimal command-line client to interact with the GoStacker backend.

Features:
- Register
- Login (receive JWT)
- Create chat room (requires JWT)
- Connect to WebSocket `/api/ws` with JWT and receive messages

Usage (Python client)

1. Install dependencies:

```bash
python3 -m pip install -r client/requirements.txt
```

2. View help:

```bash
python3 client/pyclient.py --help
```

3. Examples:

- Register:
  ```bash
  python3 client/pyclient.py register --backend http://localhost:8080
  ```
- Login:
  ```bash
  python3 client/pyclient.py login --backend http://localhost:8080
  ```
- Create room (requires token env var):
  ```bash
  export TOKEN=your_jwt
  python3 client/pyclient.py create-room --backend http://localhost:8080
  ```
- Open websocket (requires token env var):
  ```bash
  TOKEN=your_jwt python3 client/pyclient.py ws --backend http://localhost:8080
  ```

Examples:
- Register:
  ./gostacker-client register -u username -p password
- Login:
  ./gostacker-client login -u username -p password
- Create room (requires token env var):
  TOKEN=your_jwt ./gostacker-client create-room -n "room name" -m 2
- Open websocket (requires token env var):
  TOKEN=your_jwt python3 client/pyclient.py ws
