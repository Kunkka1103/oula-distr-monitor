package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	dsn            string
	pushGatewayURL string
	alertedToday   bool
)

func init() {
	// 定义命令行参数
	flag.StringVar(&dsn, "dsn", "", "PostgreSQL DSN (数据源名称)")
	flag.StringVar(&pushGatewayURL, "pushgateway-url", "", "Prometheus Pushgateway 地址")

	// 解析命令行参数
	flag.Parse()

	// 检查必要参数是否为空
	if dsn == "" {
		log.Fatal("DSN 参数不能为空")
	}
	if pushGatewayURL == "" {
		log.Fatal("Pushgateway URL 参数不能为空")
	}
}

func main() {
	for {
		checkDate()
		// 每分钟检查一次
		time.Sleep(1 * time.Minute)
	}
}

func checkDate() {
	// 获取昨天的日期
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// 连接PostgreSQL数据库
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("无法连接到数据库: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	// 执行SQL查询
	var distributorDate string
	err = db.QueryRow("SELECT distributor_date FROM distributor ORDER BY distributor_date DESC LIMIT 1").Scan(&distributorDate)
	if err != nil {
		log.Printf("查询数据库时出错: %v", err)
		return
	}

	// 获取当前时间
	now := time.Now()

	// 检查日期
	if distributorDate != yesterday {
		// 如果当前时间在8:15之后，且还没有触发过告警，则触发告警
		if now.Hour() >= 8 && now.Minute() >= 15 && !alertedToday {
			Push("distributor_date_check", "instance1", 1, pushGatewayURL)
			log.Printf("日期不匹配: 期望 %s, 但获得 %s", yesterday, distributorDate)
			alertedToday = true // 标记当天已经触发过告警
		}
	} else {
		// 如果日期匹配，清除告警并重置标志位
		Push("distributor_date_check", "instance1", 0, pushGatewayURL)
		log.Println("日期匹配，无需告警")
		alertedToday = false // 重置标志位，准备第二天的告警
	}
}

// Push 推送指标到 Prometheus Pushgateway
func Push(jobName string, instance string, value float64, url string) {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: jobName})
	gauge.Set(value)
	err := push.New(url, jobName).
		Grouping("instance", instance).
		Collector(gauge).
		Push()
	if err != nil {
		log.Printf("Push to Prometheus %s failed: %s", url, err)
	} else {
		log.Printf("Successfully pushed metric to %s: %s=%f", url, jobName, value)
	}
}
