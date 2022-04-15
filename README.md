aux-rpc
==========
基于grpc-go封装的rpc库

Installation
------------
```sh
go get github.com/dysodeng/aux-rpc
```

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
protoc --go_out=plugins=grpc:./ ./proto/demo.proto
```

创建Server端 server.go
```go
package main
import (
    "github.com/dysodeng/aux-rpc"
    demo "github.com/dysodeng/aux-rpc/proto"
    "github.com/dysodeng/aux-rpc/registry/etcdv3"
    "github.com/dysodeng/aux-rpc/rpc/service"
    "github.com/rcrowley/go-metrics"
    "log"
    "os"
    "os/signal"
)
func main() {
    
    // 服务监听地址
    listen := "127.0.0.1:9000"
	
    etcdV3Register, err := etcdv3.NewEtcdV3Registry(
        listen,
        []string{"localhost:2379"},
        etcdv3.WithNamespace("demo/grpc"),
    )
    if err != nil {
        log.Panicf("%+v", err)
    }

    rpcServer, err := auxrpc.NewServer(
        listen,
        etcdV3Register, 
        auxrpc.WithMetrics(metrics.NewMeter(), true),
    )
    if err != nil {
        log.Panicf("%+v", err)
    }
    defer func() {
        if err := recover(); err != nil {
            _ = rpcServer.Stop()
        }
    }()
    
    _ = rpcServer.Register("DemoService", &service.DemoService{}, demo.RegisterDemoServer, "")

    go func() {
        rpcServer.Serve()
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
    etcdDiscovery "github.com/dysodeng/aux-rpc/discovery/etcdv3"
    demo "github.com/dysodeng/aux-rpc/proto"
    "github.com/dysodeng/aux-rpc/rpc/service"
    clientv3 "go.etcd.io/etcd/client/v3"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "log"
    "time"
)
func main() {
    etcdClient, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"127.0.0.1:2379"},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        log.Fatalln(err)
    }
    resolverBuilder := etcdDiscovery.NewEtcdV3Builder(etcdClient, etcdDiscovery.WithNamespace("demo/grpc"))

    // 连接超时
    timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    // 连接gRPC服务
    conn, err := discovery.Discovery(
        timeoutCtx,
        resolverBuilder,
        "DemoService",
        discovery.WithLoadBalance("round_robin"),
        discovery.WithGrpcDialOption(
            grpc.WithInsecure(),
            grpc.WithBlock(),
        ),
    )
    
    demoCtx, demoCancel := context.WithDeadline(context.Background(), time.Now().Add(3 * time.Second))
    defer demoCancel()
    
    demoService := demo.NewDemoClient(conn)
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
