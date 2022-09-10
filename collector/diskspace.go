package collector

import (
	"database/sql"
	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/prometheus/common/log"
)

/**
 *  存储磁盘抓取器
 */

const (
	//?????????????????????????????????????
	//fileSystemSql = `select * from diskspace_now;`
	fileSystemSql = `select now(), * from (SELECT distinct dfhostname, dfdevice, (dfspace/1024/1024)::decimal(18,2) as "space_avail_gb" 
					FROM gp_toolkit.gp_disk_free order by dfhostname) a;`
)

var (
//	fsTotalDesc = prometheus.NewDesc(
//		prometheus.BuildFQName(namespace, subSystemNode, "fs_total_bytes"),
//		"Total bytes in the file system",
//		[]string{"hostname", "filesystem"}, nil,
//	)
//
//	fsUsedDesc = prometheus.NewDesc(
//		prometheus.BuildFQName(namespace, subSystemNode, "fs_used_bytes"),
//		"Total bytes used in the file system",
//		[]string{"hostname", "filesystem"}, nil,
//	)

	fsAvailableDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, subSystemNode, "space_avail_gb"),
		"Total GB available in the file system",
		[]string{"hostname", "device"}, nil,
	)
)

func NewDiskScraper() Scraper {
	return diskScraper{}
}

type diskScraper struct{}

func (diskScraper) Name() string {
	return "filesystem_scraper"
}

func (diskScraper) Scrape(db *sql.DB, ch chan<- prometheus.Metric, ver int) error {
	rows, err := db.Query(fileSystemSql)
	logger.Infof("Query Database: %s",fileSystemSql)

	if err != nil {
		return err
	}

	defer rows.Close()

	errs := make([]error, 0)

	for rows.Next() {
		var cTime, hostname, fs string
		var available float64

		err := rows.Scan(&cTime, &hostname, &fs, &available)

		if err != nil {
			errs = append(errs, err)
			continue
		}

//		ch <- prometheus.MustNewConstMetric(fsTotalDesc, prometheus.GaugeValue, total, hostname, fs)
//		ch <- prometheus.MustNewConstMetric(fsUsedDesc, prometheus.GaugeValue, used, hostname, fs)
		ch <- prometheus.MustNewConstMetric(fsAvailableDesc, prometheus.GaugeValue, available, hostname, fs)
	}

	return combineErr(errs...)
}
