package collector

import (
	"container/list"
	"database/sql"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

const (
	getDBNameSql      = `select datname from pg_database where datname not in ('template0','template1');`
	bloatHeapTableSql = `
		SELECT now(),current_database(),bdinspname,bdirelname,bdirelpages,bdiexppages,(
		case 
			when position('moderate' in bdidiag)>0 then 1 
			when position('significant' in bdidiag)>0 then 2 
			else 0 
		end) as bloat_state 
		FROM gp_toolkit.gp_bloat_diag ORDER BY bloat_state desc;
	`
)

var (
	bloatDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemServer, "heap_table_bloat_detail"),
		"Tables bloat detail for HashData database",
		[]string{"datname", "bdinspname", "bdirelname", "bdirelpages", "bdiexppages", "bloat_state"},
		nil,
	)
)

func NewbloatScraper() Scraper {
	return &bloatScraper{}
}

type bloatScraper struct{}

func (bloatScraper) Name() string {
	return "bloatScraper"
}

func (bloatScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {
	rows, err := db.Query(getDBNameSql)
	logger.Infof("Query Database: %s", getDBNameSql)

	if err != nil {
		return err
	}

	defer rows.Close()

	names := list.New()
	for rows.Next() {
		var dbname string
		err = rows.Scan(&dbname)

		if err != nil {
			return err
		}
		names.PushBack(dbname)
	}

	for item := names.Front(); nil != item; item = item.Next() {
		dbname := item.Value.(string)
		dataSourceName := os.Getenv("GPDB_DATA_SOURCE_URL")
		newDataSourceName := strings.Replace(dataSourceName, "/postgres", "/"+dbname, 1)
		logger.Infof("Connection string is : %s", newDataSourceName)

		conn, err := sql.Open("postgres", newDataSourceName)

		rows, err := conn.Query(bloatHeapTableSql)
		logger.Infof("Query Database: %s", bloatHeapTableSql)

		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			var datname, bdinspname, bdirelname, bdirelpages, bdiexppages, bloat_state string
			var currentTime time.Time
			err = rows.Scan(&currentTime,
				&datname,
				&bdinspname,
				&bdirelname,
				&bdirelpages,
				&bdiexppages,
				&bloat_state)

			if err != nil {
				return err
			}

			ch <- prometheus.MustNewConstMetric(bloatDesc, prometheus.GaugeValue,
				float64(currentTime.UTC().Unix()),
				datname,
				bdinspname,
				bdirelname,
				bdirelpages,
				bdiexppages,
				bloat_state)
		}

	}
	return nil
}
