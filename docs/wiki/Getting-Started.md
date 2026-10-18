# Getting Started

Requires Go **1.22+** (to build from source).

```bash
git clone https://github.com/xiaoxiaolin0918/plumelog-cli.git
cd plumelog-cli
go test ./...
go build -o plumelog .
export PLUMELOG_BASE_URL=http://127.0.0.1:8891
export PLUMELOG_LOGIN_PASSWORD='YOUR_PASSWORD'   # never commit real secrets
./plumelog doctor
./plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
# or: ./plumelog auth login --web
./plumelog query search --app demo --level ERROR -q timeout
./plumelog version
```

Install via module:

```bash
go install github.com/xiaoxiaolin0918/plumelog-cli@v0.1.2
```

Full README: https://github.com/xiaoxiaolin0918/plumelog-cli/blob/main/README.md  
中文: https://github.com/xiaoxiaolin0918/plumelog-cli/blob/main/README.zh-CN.md
