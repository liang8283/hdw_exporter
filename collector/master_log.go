package collector

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

const (
	masterLogSql = `
	select now(),a.logtime,a.loguser,a.logdatabase,a.loghost,a.logsession,a.logcmdcount,a.logseverity,a.logmessage,a.logdebug,b.logduration
    from gp_toolkit.__gp_log_master_ext a join gp_toolkit.gp_log_command_timings b
	on a.logsession = b.logsession
	and a.logcmdcount = b.logcmdcount
    where logtime > now() - interval '1 day'
	and (
		lower(logmessage)  like '%delete from%' 
		or lower(logmessage) like '%drop %' 
		or lower(logmessage) like '%truncate %' 
		or logseverity = 'ERROR'
		or b.logduration > '1 minute'
	)
	and a.logmessage not like '%gp_toolkit%'
	and a.logmessage not like 'successfully allocated xid%'
	and a.logdatabase <> 'postgres'
    order by a.logtime desc;`
)

var (
	masterLogDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemServer, "master_log_detail"),
		"Master log for HashData database",
		[]string{"logtime", "loguser", "logdatabase", "loghost", "logsession", "logcmdcount", "logseverity", "logmessage", "logdebug", "logduration"},
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
		var logtime, loguser, logdatabase, loghost, logsession, logcmdcount, logseverity, logmessage, logdebug, logduration string
		var currentTime time.Time
		err = rows.Scan(&currentTime,
			&logtime,
			&loguser,
			&logdatabase,
			&loghost,
			&logsession,
			&logcmdcount,
			&logseverity,
			&logmessage,
			&logdebug,
			&logduration)

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
			logcmdcount,
			logseverity,
			logmessage,
			logdebug,
			logduration)
	}

	return nil
}
