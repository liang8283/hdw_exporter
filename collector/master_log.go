package collector

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

const (
	masterLogSql = `
	select now(),logtime,loguser,logdatabase,loghost,logsession,logmessage 
    from gp_toolkit.__gp_log_master_ext
    where logtime > now() - interval '1 day'
	and (lower(logmessage)  like '%delete from%' or lower(logmessage) like '%drop %' or lower(logmessage) like '%truncate %')
	and logmessage not like '%gp_toolkit%'
    order by logtime desc;`
)

var (
	masterLogDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemServer, "master_log_detail"),
		"Master log for HashData database",
		[]string{"logtime", "loguser", "logdatabase", "loghost", "logsession", "logmessage"},
		nil,
	)
)

func NewMasterLogScraper() Scraper {
	return &masterLogScraper{}
}

type masterLogScraper struct{}

func (masterLogScraper) Name() string {
	return "masterLogScraper"
}

func (masterLogScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {

	rows, err := db.Query(masterLogSql)
	logger.Infof("Query Database: %s", masterLogSql)

	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var logtime, loguser, logdatabase, loghost, logsession, logmessage string
		var currentTime time.Time
		err = rows.Scan(&currentTime,
			&logtime,
			&loguser,
			&logdatabase,
			&loghost,
			&logsession,
			&logmessage)

		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(masterLogDesc, prometheus.GaugeValue,
			float64(currentTime.UTC().Unix()),
			logtime,
			loguser,
			logdatabase,
			loghost,
			logsession,
			logmessage)
	}

	return nil
}
