package collector

import (
	"database/sql"
	"time"
	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

const (
	sessionMemorySql_v6 = `
		select now(),b.pid,b.sess_id,b.datname,b.usename, max(vmem_mb) as vmem_max_seg,round(avg(vmem_mb)) as vmem_avg,sum(vmem_mb) as vmem_total, b.query
		from session_state.session_level_memory_consumption a join pg_stat_activity b
		on a.sess_id = b.sess_id
		where b.pid <> pg_backend_pid()
		and b.datname is not null
		group by b.pid,b.sess_id,b.datname,b.usename, b.query`
	sessionMemorySql_v5 = `
		select now(),b.procpid,a.sess_id,a.datname,a.usename, max(vmem_mb) as vmem_max_seg,round(avg(vmem_mb)) as vmem_avg,sum(vmem_mb) as vmem_total, a.current_query
		from session_state.session_level_memory_consumption a join pg_stat_activity b
		on a.sess_id = b.sess_id
		where b.procpid <> pg_backend_pid()
		group by b.procpid,a.sess_id,a.datname,a.usename, a.current_query`
)

var (
	sessionMemoryDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemServer, "session_memory_detail"),
		"Sessions memory usage detail for all running sessions",
		[]string{"pid", "sess_id", "datname", "usename", "vmem_max_seg", "vmem_avg", "vmem_total", "query"},
		nil,
	)
)

func NewSessionMemoryScraper() Scraper {
	return &sessionMemoryScraper{}
}

type sessionMemoryScraper struct{}

func (sessionMemoryScraper) Name() string {
	return "sessionMemoryScraper"
}

func (sessionMemoryScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {
	sessionMemorySql :=sessionMemorySql_v6;
	if ver > 3 && ver < 6{
		sessionMemorySql=sessionMemorySql_v5;
	}

	rows, err := db.Query(sessionMemorySql)
	logger.Infof("Query Database: %s",sessionMemorySql)

	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var pid,sess_id,vmem_max_seg,vmem_avg,vmem_total,query string
		var datname, usename sql.NullString
		var currentTime time.Time
//		var count int64
		err = rows.Scan(&currentTime,
			&datname,
			&pid,
			&sess_id,
			&usename,
			&vmem_max_seg,
			&vmem_avg,
			&vmem_total,
			&query)

		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(sessionMemoryDesc, prometheus.GaugeValue, 
			float64(currentTime.UTC().Unix()),
			datname.String, 
			pid,
			sess_id,
			usename.String,
			vmem_max_seg,
			vmem_avg,
			vmem_total,
			query)
	}

	return nil
}
