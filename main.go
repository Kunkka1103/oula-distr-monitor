package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"strings"
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

	log.Println("初始化完成：DSN 和 Pushgateway URL 参数已设置")
}

func main() {
	log.Println("程序开始运行...")

	for {
		log.Println("开始新一轮日期检查...")
		checkDate()
		log.Println("日期检查完成，等待 1 分钟后重新检查...")
		// 每分钟检查一次
		time.Sleep(1 * time.Minute)
	}
}

func checkDate() {
	// 获取昨天的日期
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	log.Printf("获取到的昨天日期为：%s", yesterday)

	// 连接PostgreSQL数据库
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("无法连接到数据库: %v", err)
		os.Exit(1)
	}
	defer db.Close()

	log.Println("成功连接到数据库")

	// 执行SQL查询并获取日期部分
	var distributorDate string
	err = db.QueryRow("SELECT distributor_date FROM distributor ORDER BY distributor_date DESC LIMIT 1").Scan(&distributorDate)
	if err != nil {
		log.Printf("查询数据库时出错: %v", err)
		return
	}

	// 提取日期部分（忽略时间部分）
	distributorDate = strings.Split(distributorDate, "T")[0]
	log.Printf("查询到的 distributor_date 为：%s", distributorDate)

	// 获取当前时间
	now := time.Now()
	log.Printf("当前时间为：%s", now.Format("2006-01-02 15:04:05"))

	// 检查日期
	if distributorDate != yesterday {
		// 如果当前时间在8:15之后，且还没有触发过告警，则触发告警
		if now.Hour() >= 8 && now.Minute() >= 30 && !alertedToday {
			log.Printf("日期不匹配: 期望 %s, 但获得 %s，触发告警", yesterday, distributorDate)
			Push("oula_distributor_date_check", 1, pushGatewayURL)
			alertedToday = true // 标记当天已经触发过告警
		} else {
			log.Println("日期不匹配，但尚未到8:30，暂不触发告警")
		}
	} else {
		// 如果日期匹配，清除告警并重置标志位
		log.Println("日期匹配，清除告警")
		Push("oula_distributor_date_check", 0, pushGatewayURL)
		alertedToday = false // 重置标志位，准备第二天的告警
	}
}

// Push 推送指标到 Prometheus Pushgateway
func Push(jobName string, value float64, url string) {
	log.Printf("开始推送指标：%s=%f 到 %s", jobName, value, url)
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: jobName})
	gauge.Set(value)
	err := push.New(url, jobName).
		Collector(gauge).
		Push()
	if err != nil {
		log.Printf("推送到 Prometheus %s 失败: %v", url, err)
	} else {
		log.Printf("成功推送指标到 %s: %s=%f", url, jobName, value)
	}
}
