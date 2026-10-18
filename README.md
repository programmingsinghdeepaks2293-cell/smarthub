# SmartHub

Single-pane control panel for Tapo plugs and Philips Hue bulbs.
Go backend, React + TypeScript frontend, Python sidecar for Tapo.

## Run

    # 1. Tapo sidecar
    cd tapo-bridge && pip install -r requirements.txt
    TAPO_EMAIL=you@example.com TAPO_PASSWORD=secret uvicorn main:app --port 8765

    # 2. Go backend (from the project root)
    go mod tidy
    cd cmd/server && go generate && go run .

    # 3. React frontend
    cd frontend && npm install && npm run dev    # http://localhost:5173

Edit devices.json to set your plug IPs and Hue details.
Add a new device by copying plugins/example to plugins/<name>, then run `go generate` in cmd/server.
