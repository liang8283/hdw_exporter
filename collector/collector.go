package collector

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"hdw-exporter/stopwatch"
	logger "github.com/prometheus/common/log"
	"os"
	"sync"
	"time"
)

const verMajorSql=`select (select regexp_matches((select (select regexp_matches((select version()), 'Greenplum Database \d{1,}\.\d{1,}\.\d{1,}'))[1] as version), '\d{1,}'))[1];`


type HdwCollector struct {
	mu sync.Mutex

	db       *sql.DB
	ver       int
	metrics  *ExporterMetrics
	scrapers []Scraper
}


func NewCollector(enabledScrapers []Scraper) *HdwCollector {
	return &HdwCollector{
		metrics:  NewMetrics(),
		scrapers: enabledScrapers,
	}
}

func (c *HdwCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.scrape(ch)

	ch <- c.metrics.totalScraped
	ch <- c.metrics.totalError
	ch <- c.metrics.scrapeDuration
	ch <- c.metrics.hdwUp
}


func (c *HdwCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metrics.hdwUp.Desc()
	ch <- c.metrics.scrapeDuration.Desc()
	ch <- c.metrics.totalScraped.Desc()
	ch <- c.metrics.totalError.Desc()
}

func (c *HdwCollector) scrape(ch chan<- prometheus.Metric) {
	start := time.Now()
	watch := stopwatch.New("scrape")


	c.metrics.totalScraped.Inc()
	watch.MustStart("check connections")
	err := c.checkHdwConn()
	watch.MustStop()
	if err != nil {
		c.metrics.totalError.Inc()
		c.metrics.scrapeDuration.Set(time.Since(start).Seconds())
		c.metrics.hdwUp.Set(0)

		logger.Errorf("check database connection failed, error:%v", err)

		return
	}

	defer c.db.Close()

	logger.Info("check connections ok!")
	c.metrics.hdwUp.Set(1)


	for _, scraper := range c.scrapers {
		logger.Info("#### scraping start : " + scraper.Name())
		watch.MustStart("scraping: " + scraper.Name())
		err := scraper.Scrape(c.db, ch, c.ver)
		watch.MustStop()
		if err != nil {
			logger.Errorf("get metrics for scraper:%s failed, error:%v", scraper.Name(), err.Error())
		}
		logger.Info("#### scraping end : " + scraper.Name())
	}

	c.metrics.scrapeDuration.Set(time.Since(start).Seconds())

	logger.Info(fmt.Sprintf("prometheus scraped hdw exporter successfully at %v, detail elapsed:%s", time.Now(), watch.PrettyPrint()))
}


func (c *HdwCollector) checkHdwConn() (err error) {
	if c.db == nil {
		return c.getHdwConnection()
	}

	if err = c.getHdwMajorVersion(c.db); err == nil {
		return nil
	} else {
		_ = c.db.Close()
		c.db = nil
		return c.getHdwConnection()
	}
}


func (c *HdwCollector) getHdwConnection() error {

	dataSourceName := os.Getenv("GPDB_DATA_SOURCE_URL")

	db, err := sql.Open("postgres", dataSourceName)

	if err != nil {
		return err
	}

	if err = c.getHdwMajorVersion(db); err != nil {
		_ = db.Close()
		return err
	}

	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)

	c.db = db

	return nil
}


func (c *HdwCollector) getHdwMajorVersion(db *sql.DB) error {
	err := db.Ping()

	if err != nil {
		return err
	}

	rows, err := db.Query(verMajorSql)

	if err != nil {
		return err
	}

	for rows.Next() {
		var verMajor int
		errC := rows.Scan(&verMajor)
		if errC != nil {
			return errC
		}

		c.ver=verMajor
	}

	defer rows.Close()

	return nil
}
