package collector

import (
	"container/list"
	"context"
	"database/sql"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

const (
	//	getDBNameSql = `select datname from pg_database where datname not in ('template0','template1');`
	create_data_skew_fn_sql = `
	CREATE OR REPLACE FUNCTION public.fn_get_skew(out schema_name      varchar,
		out table_name       varchar,
		out total_size_GB    numeric(15,2),
		out seg_min_size_GB  numeric(15,2),
		out seg_max_size_GB  numeric(15,2),
		out seg_avg_size_GB  numeric(15,2),
		out seg_gap_min_max_percent numeric(6,2),
		out seg_gap_min_max_GB      numeric(15,2),
		out nb_empty_seg     int) RETURNS SETOF record AS
	$$
	DECLARE
		v_function_name text := 'fn_get_skew';
		v_location int;
		v_sql text;
		v_db_oid text;
		v_num_segments numeric;
		v_skew_amount numeric;
		v_res record;
	BEGIN
		v_location := 1000;
		SELECT oid INTO v_db_oid
		FROM pg_database
		WHERE datname = current_database();

		v_location := 2200;
		v_sql := 'DROP EXTERNAL TABLE IF EXISTS public.db_files_ext';

		v_location := 2300;
		EXECUTE v_sql;

		v_location := 3000;
		v_sql := 'CREATE EXTERNAL WEB TABLE public.db_files_ext ' ||
		'(segment_id int, relfilenode text, filename text, ' ||
		'size numeric) ' ||
		'execute E''ls -l $GP_SEG_DATADIR/base/' || v_db_oid ||
		' | ' ||
		'grep gpadmin | ' ||
		E'awk {''''print ENVIRON["GP_SEGMENT_ID"] "\\t" $9 "\\t" ' ||
		'ENVIRON["GP_SEG_DATADIR"] "/' || v_db_oid ||
		E'/" $9 "\\t" $5''''}'' on all ' || 'format ''text''';

		v_location := 3100;
		EXECUTE v_sql;

		v_location := 4000;
		for v_res in (
		select  sub.vschema_name,
		sub.vtable_name,
		(sum(sub.size)/(1024^3))::numeric(15,2) AS vtotal_size_GB,
		--Size on segments
		(min(sub.size)/(1024^3))::numeric(15,2) as vseg_min_size_GB,
		(max(sub.size)/(1024^3))::numeric(15,2) as vseg_max_size_GB,
		(avg(sub.size)/(1024^3))::numeric(15,2) as vseg_avg_size_GB,
		--Percentage of gap between smaller segment and bigger segment
		(100*(max(sub.size) - min(sub.size))/greatest(max(sub.size),1))::numeric(6,2) as vseg_gap_min_max_percent,
		((max(sub.size) - min(sub.size))/(1024^3))::numeric(15,2) as vseg_gap_min_max_GB,
		count(sub.size) filter (where sub.size = 0) as vnb_empty_seg
		from (
		SELECT  n.nspname AS vschema_name,
		c.relname AS vtable_name,
		db.segment_id,
		sum(db.size) AS size
		FROM ONLY public.db_files_ext db
		JOIN pg_class c ON split_part(db.relfilenode, '.'::text, 1) = c.relfilenode::text
		JOIN pg_namespace n ON c.relnamespace = n.oid
		WHERE c.relkind = 'r'::"char"
		and n.nspname not in ('pg_catalog','information_schema','gp_toolkit')
		and not n.nspname like 'pg_temp%'
		GROUP BY n.nspname, c.relname, db.segment_id
		) sub
		group by 1,2
		--Extract only table bigger than 1 GB
		--   and with a skew greater than 20%
		having sum(sub.size)/(1024^3) > 1
		and (100*(max(sub.size) - min(sub.size))/greatest(max(sub.size),1))::numeric(6,2) > 20
		order by vtotal_size_GB desc, vseg_gap_min_max_percent desc
		limit 100 ) loop
		schema_name         = v_res.vschema_name;
		table_name          = v_res.vtable_name;
		total_size_GB       = v_res.vtotal_size_GB;
		seg_min_size_GB     = v_res.vseg_min_size_GB;
		seg_max_size_GB     = v_res.vseg_max_size_GB;
		seg_avg_size_GB     = v_res.vseg_avg_size_GB;
		seg_gap_min_max_percent = v_res.vseg_gap_min_max_percent;
		seg_gap_min_max_GB  = v_res.vseg_gap_min_max_GB;
		nb_empty_seg        = v_res.vnb_empty_seg;
		return next;
		end loop;

		v_location := 4100;
		v_sql := 'DROP EXTERNAL TABLE IF EXISTS public.db_files_ext';

		v_location := 4200;
		EXECUTE v_sql;

		return;
		EXCEPTION
		WHEN OTHERS THEN
		RAISE EXCEPTION '(%:%:%)', v_function_name, v_location, sqlerrm;
	END;
	$$
	language plpgsql;`
	get_data_skew_sql = `select * from public.fn_get_skew();`
)

var (
	ctx          context.Context
	dataSkewDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemServer, "dataSkew_detail"),
		"Tables > 20% data skew across segments.",
		[]string{"schema_name", "table_name", "total_size_gb", "seg_min_size_gb", "seg_max_size_gb", "seg_avg_size_gb", "seg_gap_min_max_percent", "seg_gap_min_max_gb", "nb_empty_seg"},
		nil,
	)
)

func NewDataSkewScraper() Scraper {
	return &DataSkewScraper{}
}

type DataSkewScraper struct{}

func (DataSkewScraper) Name() string {
	return "DataSkewScraper"
}

func (DataSkewScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {
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
		defer conn.Close()

		_, err = conn.Exec(create_data_skew_fn_sql)
		logger.Infof("Query Database: %s", create_data_skew_fn_sql)

		if err != nil {
			return err
		}

		_, err = conn.Exec("SET client_min_messages TO WARNING;")
		logger.Infof("Query Database: %s", "SET client_min_messages TO WARNING;")

		if err != nil {
			return err
		}

		rows, err := conn.Query(get_data_skew_sql)
		logger.Infof("Query Database: %s", get_data_skew_sql)

		if err != nil {
			logger.Errorf("get metrics for scraper, error:%v", err.Error())
			return err
		}

		defer rows.Close()

		//	if err = tx.Commit(); err != nil {
		//		return err
		//	}

		for rows.Next() {
			var schema_name, table_name, total_size_gb, seg_min_size_gb, seg_max_size_gb, seg_avg_size_gb,
				seg_gap_min_max_percent, seg_gap_min_max_gb, nb_empty_seg string
			err = rows.Scan(&schema_name,
				&table_name,
				&total_size_gb,
				&seg_min_size_gb,
				&seg_max_size_gb,
				&seg_avg_size_gb,
				&seg_gap_min_max_percent,
				&seg_gap_min_max_gb,
				&nb_empty_seg)

			if err != nil {
				return err
			}

			ch <- prometheus.MustNewConstMetric(dataSkewDesc, prometheus.GaugeValue,
				float64(time.Now().UTC().Unix()),
				schema_name,
				table_name,
				total_size_gb,
				seg_min_size_gb,
				seg_max_size_gb,
				seg_avg_size_gb,
				seg_gap_min_max_percent,
				seg_gap_min_max_gb,
				nb_empty_seg)
		}
	}
	return nil
}
