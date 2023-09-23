package collector

import (
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

const (
//	pgActivitySql_v6 = `
//		select 
//			now(),
//			datname,
//			pid,
//			sess_id,
//			usename,
//			application_name,
//			client_addr,
//			backend_start,
//			least(query_start,xact_start) as start_time,
//			round(extract(epoch FROM (now() - query_start))) as duration,
//			waiting,
//			query,
//			wait_event,
//			rsgname,
//			rsgqueueduration,
//			count(*)::float 
//		from pg_stat_activity
//		where pid <> pg_backend_pid()
//		group by datname,pid,sess_id,usename,application_name,client_addr,start_time,backend_start,duration,waiting,query,wait_event,rsgname,rsgqueueduration
//		order by start_time;`
	pgActivitySql_v6 = `
		select 
		now(),
		datname,
		pid,
		sess_id,
		usename,
		application_name,
		client_addr,
		backend_start,
		least(query_start,xact_start) as start_time,
		round(extract(epoch FROM (now() - query_start))) as duration,
		wait_event,
		query,
		wait_event_type,
		rsgname,
		count(*)::float 
		from pg_stat_activity
		where pid <> pg_backend_pid()
		group by datname,pid,sess_id,usename,application_name,client_addr,start_time,backend_start,duration,wait_event,query,wait_event_type,rsgname
		order by start_time;
	`
	pgActivitySql_v5 = `
		select 
			now(),
			datname,
			procpid,
			sess_id,
			usename,
			application_name,
			client_addr,
			backend_start,
			least(query_start,xact_start) as start_time,
			round(extract(epoch FROM (now() - query_start))) as duration,
			waiting,
			current_query,
			waiting_reason,
			rsgname,
			count(*)::float
		from pg_stat_activity
		where procpid <> pg_backend_pid()
		group by datname,procpid,sess_id,usename,application_name,client_addr,start_time,backend_start,duration,waiting,current_query,waiting_reason,rsgname
		order by start_time;`
)

var (
	activityDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemServer, "activity_detail"),
		"Processes detail for HashData database",
		[]string{"datname", "pid", "sess_id", "usename", "application_name", "client_addr", "backend_start", "start_time", "duration", "wait_event", "query", "wait_event_type", "rsgname"},
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
	if ver > 3 && ver < 6 {
		activitySql = pgActivitySql_v5
	}

	rows, err := db.Query(activitySql)
	logger.Infof("Query Database: %s", activitySql)

	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var pid, sess_id, application_name, query, rsgname string
		var datname, usename, start_time, backend_start, client_addr, duration, wait_event, wait_event_type sql.NullString
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
			&backend_start,
			&duration,
			&wait_event,
			&query,
			&wait_event_type,
			&rsgname,
			&count)

		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(activityDesc, prometheus.GaugeValue,
			float64(currentTime.UTC().Unix()),
			datname.String,
			pid,
			sess_id,
			usename.String,
			application_name,
			client_addr.String,
			start_time.String,
			backend_start.String,
			duration.String,
			wait_event.String,
			query,
			wait_event_type.String,
			rsgname)
	}

	return nil
}
