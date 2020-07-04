# 服务注册启动脚本
rm register
go build -o register main.go
./register -consul.host 192.168.170.128 -consul.port 8500 -service.host 192.168.170.128 -service.port 9000
./register -consul.host 192.168.170.128 -consul.port 8500 -service.host 192.168.170.128 -service.port 9009