package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/afex/hystrix-go/hystrix"
	"github.com/go-kit/kit/log"
	"github.com/hashicorp/consul/api"
	"github.com/openzipkin/zipkin-go"
	zipkinhttpsvr "github.com/openzipkin/zipkin-go/middleware/http"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"

	kit_svc "gateway/service"
)

func main() {

	// 创建环境变量
	var (
		consulHost = flag.String("consul.host", "192.168.170.128", "consul server ip address")
		consulPort = flag.String("consul.port", "8500", "consul server port")
		zipkinURL  = flag.String("zipkin.url", "http://192.168.170.128:9411/api/v2/spans", "Zipkin server url")
	)
	flag.Parse()

	//创建日志组件
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	var zipkinTracer *zipkin.Tracer
	{
		var (
			err           error
			hostPort      = "localhost:9090"
			serviceName   = "gateway-service"
			useNoopTracer = (*zipkinURL == "")
			reporter      = zipkinhttp.NewReporter(*zipkinURL)
		)
		defer reporter.Close()
		zEP, _ := zipkin.NewEndpoint(serviceName, hostPort)
		zipkinTracer, err = zipkin.NewTracer(
			reporter, zipkin.WithLocalEndpoint(zEP), zipkin.WithNoopTracer(useNoopTracer),
		)
		if err != nil {
			logger.Log("err", err)
			os.Exit(1)
		}
		if !useNoopTracer {
			logger.Log("tracer", "Zipkin", "type", "Native", "URL", *zipkinURL)
		}
	}

	// 创建consul api客户端
	consulConfig := api.DefaultConfig()
	consulConfig.Address = "http://" + *consulHost + ":" + *consulPort
	consulClient, err := api.NewClient(consulConfig)
	if err != nil {
		logger.Log("err", err)
		os.Exit(1)
	}

	tags := map[string]string{
		"component": "gateway_server",
	}

	hystrixRouter := kit_svc.Routes(consulClient, zipkinTracer, "Circuit Breaker:Service unavailable", logger)

	handler := zipkinhttpsvr.NewServerMiddleware(
		zipkinTracer,
		zipkinhttpsvr.SpanName("gateway"),
		zipkinhttpsvr.TagResponseSize(true),
		zipkinhttpsvr.ServerTags(tags),
	)(hystrixRouter)

	errc := make(chan error)

	//启用hystrix实时监控，监听端口为9010
	hystrixStreamHandler := hystrix.NewStreamHandler()
	hystrixStreamHandler.Start()
	go func() {
		errc <- http.ListenAndServe(net.JoinHostPort("", "9010"), hystrixStreamHandler)
	}()

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	//开始监听
	go func() {
		logger.Log("transport", "HTTP", "addr", "9090")
		errc <- http.ListenAndServe(":9090", handler)
	}()

	// 开始运行，等待结束
	logger.Log("exit", <-errc)
}
