package collector

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

const (
	pgActivitySql_v6 = `
		select 
			now(),
			datname,
			pid,
			sess_id,
			usename,
			application_name,
			client_addr,
			least(query_start,xact_start) as start_time,
			round(extract(epoch FROM (now() - query_start))) as duration,
			waiting,
			query,
			waiting_reason,
			rsgname,
			rsgqueueduration,
			count(*)::float 
		from pg_stat_activity
		where pid <> pg_backend_pid()
		group by datname,pid,sess_id,usename,application_name,client_addr,start_time,duration,waiting,query,waiting_reason,rsgname,rsgqueueduration
		order by start_time;`
	pgActivitySql_v5 = `
		select 
			now(),
			datname,
			procpid,
			sess_id,
			usename,
			application_name,
			client_addr,
			least(query_start,xact_start) as start_time,
			round(extract(epoch FROM (now() - query_start))) as duration,
			waiting,
			current_query,
			waiting_reason,
			rsgname,
			rsgqueueduration,
			count(*)::float
		from pg_stat_activity
		where procpid <> pg_backend_pid()
		group by datname,procpid,sess_id,usename,application_name,client_addr,start_time,duration,waiting,current_query,waiting_reason,rsgname,rsgqueueduration
		order by start_time;`
)

var (
	activityDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemServer, "activity_detail"),
		"Processes detail for HashData database",
		[]string{"datname", "pid", "sess_id", "usename", "application_name", "client_addr", "start_time", "duration", "waiting", "query", "waiting_reason", "rsgname", "rsgqueueduration"},
		nil,
	)
)

func NewActivityScraper() Scraper {
	return &activityScraper{}
}

type activityScraper struct{}

func (activityScraper) Name() string {
	return "activityScraper"
}

func (activityScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {
	activitySql := pgActivitySql_v6
	if ver < 6 {
		activitySql = pgActivitySql_v5
	}

	rows, err := db.Query(activitySql)
	logger.Infof("Query Database: %s", activitySql)

	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var pid, sess_id, datname, usename, application_name, duration,
			waiting, query, rsgname string
		var start_time, client_addr, waiting_reason, rsgqueueduration sql.NullString
		var currentTime time.Time
		var count int64
		err = rows.Scan(&currentTime,
			&datname,
			&pid,
			&sess_id,
			&usename,
			&application_name,
			&client_addr,
			&start_time,
			&duration,
			&waiting,
			&query,
			&waiting_reason,
			&rsgname,
			&rsgqueueduration,
			&count)

		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(activityDesc, prometheus.GaugeValue,
			float64(currentTime.UTC().Unix()),
			datname,
			pid,
			sess_id,
			usename,
			application_name,
			client_addr.String,
			start_time.String,
			duration,
			waiting,
			query,
			waiting_reason.String,
			rsgname,
			rsgqueueduration.String)
	}

	return nil
}
