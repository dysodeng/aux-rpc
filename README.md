aux-rpc
==========
基于grpc-go封装的rpc库

Usage
-----
创建服务proto: demo.proto
```proto
syntax = "proto3";

package proto;

message Request {
    int64 uid = 1;
}

message Response {
    int64 id = 1;
    string username = 2;
}

service Demo {
    rpc UserInfo(Request) returns (Response);
}
```

编译proto
```sh
protoc --go_out=plugins=grpc:./ ./demo.proto
```

创建Server端 server.go
```go
package main
import (
	"github.com/dysodeng/aux-rpc"
	"github.com/dysodeng/aux-rpc/register"
	demo "github.com/dysodeng/aux-rpc/rpc/proto"
	"github.com/dysodeng/aux-rpc/rpc/service"
	"github.com/rcrowley/go-metrics"
	"log"
	"os"
	"os/signal"
)
func main() {
    etcdV3Register := &register.EtcdV3Register{
        ServiceAddress: "127.0.0.1:9000",
        EtcdServers:    []string{"localhost:2379"},
        BasePath:       "demo/rpc",
        Lease:          5,
        Metrics: 		metrics.NewMeter(),
    }
    
    rpcServer := drpc.NewServer(etcdV3Register)
    defer func() {
        if err := recover(); err != nil {
            _ = rpcServer.Stop()
        }
    }()
    
    _ = rpcServer.Register("DemoService", &service.DemoService{}, demo.RegisterDemoServer, "")

    go func() {
        rpcServer.Serve(ip + ":9000")
    }()

    // 等待中断信号以优雅地关闭服务器
    quit := make(chan os.Signal)
    signal.Notify(quit, os.Interrupt)
    <-quit
    log.Println("shutdown rpc server ...")
    _ = rpcServer.Stop()
}
```

创建客户端 client.go
```go
package main
import (
    "context"
    "github.com/dysodeng/aux-rpc/discovery"
    demo "github.com/dysodeng/aux-rpc/rpc/proto"
    "github.com/dysodeng/aux-rpc/rpc/service"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "log"
    "time"
)
func main() {
    d, err := discovery.NewEtcdV3Discovery([]string{"127.0.0.1:2379"}, "demo/rpc")
    if err != nil {
        log.Fatalln(err)
    }
    defer d.Close()
    
    demoCtx, demoCancel := context.WithDeadline(context.Background(), time.Now().Add(3 * time.Second))
    defer demoCancel()
    
    demoConn := d.Conn("DemoService")
    demoService := demo.NewDemoClient(demoConn)
    demoRes, err := demoService.UserInfo(demoCtx, &demo.Request{Uid: 1})
    if err != nil {
        //获取错误状态
        state, ok := status.FromError(err)
        if ok {
            //判断是否为调用超时
            if state.Code() == codes.DeadlineExceeded {
                log.Fatalln("UserInfo timeout!")
            }
            log.Println(err)
        }
    } else {
        log.Printf("%+v", demoRes)
        log.Println(demoRes.Username)
    }
}
```

启动服务端
```sh
go run server.go
```
客户端访问
```sh
go run client.go
```

Installation
------------
```sh
go get github.com/dysodeng/aux-rpc
```


