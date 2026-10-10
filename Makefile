# CodeCounter 常用开发命令
#
# Windows 环境说明：
#   若未安装 GNU Make，可直接执行等价的 go 命令，
#   例如 make build 等价于 go build -o codecounter.exe ./cmd/codecounter

BINARY  := codecounter.exe
VERSION ?= 0.1.0-dev
LDFLAGS := -X main.version=$(VERSION)

.PHONY: all build build-gui test vet fmt tidy clean run winres

all: fmt vet test build

## build: 构建命令行版本
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/codecounter

## build-gui: 构建发布版本，隐藏控制台窗口
build-gui:
	go build -ldflags "-H=windowsgui $(LDFLAGS)" -o $(BINARY) ./cmd/codecounter

## winres: 重新生成 Windows 资源文件（应用清单）
## 说明：cmd/codecounter 下的 .syso 文件由 winres/app.manifest 生成，
##       已纳入版本管理，只有修改清单后才需要重新执行。
winres:
	go generate ./cmd/codecounter

## test: 运行全部单元测试
test:
	go test ./...

## vet: 静态检查
vet:
	go vet ./...

## fmt: 格式化代码
fmt:
	gofmt -l -w .

## tidy: 整理依赖
tidy:
	go mod tidy

## clean: 清理构建产物
clean:
	go clean
	-rm -f $(BINARY)

## run: 统计当前项目自身
run:
	go run ./cmd/codecounter --path .
