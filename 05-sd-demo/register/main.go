package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	kit_svc "register/service"

	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	_ "github.com/juju/ratelimit"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

func main() {

	// 定义参数
	var (
		consulHost  = flag.String("consul.host", "", "consul ip address")
		consulPort  = flag.String("consul.port", "", "consul port")
		serviceHost = flag.String("service.host", "", "service ip address")
		servicePort = flag.String("service.port", "", "service port")
	)

	flag.Parse()

	ctx := context.Background()
	errChan := make(chan error)

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	// 普罗米修斯
	fieldKeys := []string{"method"}
	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "raysonxin",
		Subsystem: "arithmetic_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)

	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "raysonxin",
		Subsystem: "arithemetic_service",
		Name:      "request_latency",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)

	var svc kit_svc.Service
	svc = kit_svc.ArithmeticService{}

	// 装饰器模式，添加日志中间件
	svc = kit_svc.LoggingMiddleware(logger)(svc)
	// 装饰器模式，添加普罗米修斯中间件
	svc = kit_svc.Metrics(requestCount, requestLatency)(svc)

	endpoint := kit_svc.MakeArithmeticEndpoint(svc)

	// 限流方式一
	// add ratelimit,refill every 3 second,set capacity 3
	// 使用go-kit内置的中间件模式，添加ratelimit，每3秒设置桶的空间大小为3
	// ratebucket := ratelimit.NewBucket(time.Second*3, 3)
	// endpoint = kit_svc.NewTokenBucketLimitterWithJuju(ratebucket)(endpoint)

	// 限流方式二
	//add ratelimit,refill every 3 second,set capacity 3
	// 使用go-kit内置的中间件模式，使用内置的限流方法添加ratelimit，每3秒设置桶的空间大小为3
	ratebucket := rate.NewLimiter(rate.Every(time.Second*3), 3)
	endpoint = kit_svc.NewTokenBucketLimitterWithBuildIn(ratebucket)(endpoint)

	//创建健康检查的Endpoint，未增加限流
	healthEndpoint := kit_svc.MakeHealthCheckEndpoint(svc)

	//把算术运算Endpoint和健康检查Endpoint封装至ArithmeticEndpoints
	endpts := kit_svc.ArithmeticEndpoints{
		ArithmeticEndpoint:  endpoint,
		HealthCheckEndpoint: healthEndpoint,
	}

	//创建http.Handler
	r := kit_svc.MakeHttpHandler(ctx, endpts, logger)

	//创建注册对象
	registar := kit_svc.Register(*consulHost, *consulPort, *serviceHost, *servicePort, logger)

	go func() {
		fmt.Println("Http Server start at port:" + *servicePort)

		//启动前执行注册
		registar.Register()
		handler := r
		errChan <- http.ListenAndServe(":"+*servicePort, handler)
	}()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errChan <- fmt.Errorf("%s", <-c)
	}()

	erro := <-errChan

	//服务退出取消注册
	registar.Deregister()

	fmt.Println(erro)
}
