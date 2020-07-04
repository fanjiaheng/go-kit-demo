package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	kit_svc "04-metrics-demo/service"

	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	_ "github.com/juju/ratelimit"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"
)

func main() {

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

	r := kit_svc.MakeHttpHandler(ctx, endpoint, logger)

	go func() {
		fmt.Println("Http Server start at port:9000")
		handler := r
		errChan <- http.ListenAndServe(":9000", handler)
	}()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errChan <- fmt.Errorf("%s", <-c)
	}()

	fmt.Println(<-errChan)
}
