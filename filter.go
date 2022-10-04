package xspider

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"

	"go.uber.org/zap"
)

type DupeFilter interface {
	FromSpider(spider *Spider)
	RequestSeen(request *Request) bool
	RequestFingerprint(request *Request) string
	Close()
	Log(request *Request)
}

type DefaultDupeFilter struct {
	moduleName   string
	logger       *zap.SugaredLogger
	stats        StatsCollector
	fingerprints Setter
}

func (df *DefaultDupeFilter) FromSpider(spider *Spider) {
	df.moduleName = "dupefilter"
	df.logger = spider.Log.With("module_name", df.moduleName)
	df.stats = spider.Stats
	df.fingerprints = NewSet(0)
}

func (df *DefaultDupeFilter) RequestFingerprint(request *Request) string {
	header := []string{}
	if len(*request.Headers) != 0 {
		for k, v := range *request.Headers {
			tem := []string{}
			for _, v := range v {
				tem = append(tem, strings.ToLower(v))
			}
			sort.Strings(tem)
			header = append(header, strings.ToLower(k)+strings.Join(tem, ""))
		}
	}
	sort.Strings(header)
	var build strings.Builder
	build.WriteString(strings.ToLower(request.Method))
	build.WriteString(strings.ToLower(request.Url.String()))
	p := ReadRequestBody(request)
	build.Write(p)
	build.WriteString(strings.Join(header, ""))
	o := sha1.New()
	o.Write([]byte(build.String()))
	return hex.EncodeToString(o.Sum(nil))
}

func (df *DefaultDupeFilter) RequestSeen(request *Request) bool {
	fp := df.RequestFingerprint(request)
	if df.fingerprints.Has(fp) {
		return true
	}
	df.fingerprints.Add(fp)
	return false
}

func (df *DefaultDupeFilter) Log(request *Request) {
	NewRequestLogger(df.logger, request).Debug("Request已滤除")

	df.stats.IncValue("dupefilter/filtered", 1, 0)
}

func (df *DefaultDupeFilter) Close() {}
