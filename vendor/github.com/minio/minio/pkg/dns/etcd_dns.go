/*
 * MinIO Cloud Storage, (C) 2018 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v6/pkg/set"

	"github.com/coredns/coredns/plugin/etcd/msg"
	etcd "github.com/coreos/etcd/clientv3"
)

// ErrNoEntriesFound - Indicates no entries were found for the given key (directory)
var ErrNoEntriesFound = errors.New("No entries found for this key")

const etcdPathSeparator = "/"

// create a new coredns service record for the bucket.
func newCoreDNSMsg(ip string, port string, ttl uint32) ([]byte, error) {
	return json.Marshal(&SrvRecord{
		Host:         ip,
		Port:         json.Number(port),
		TTL:          ttl,
		CreationDate: time.Now().UTC(),
	})
}

// List - Retrieves list of DNS entries for the domain.
func (c *CoreDNS) List() ([]SrvRecord, error) {
	var srvRecords []SrvRecord
	for _, domainName := range c.domainNames {
		key := msg.Path(fmt.Sprintf("%s.", domainName), c.prefixPath)
		records, err := c.list(key)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if record.Key == "" {
				continue
			}
			srvRecords = append(srvRecords, record)
		}
	}
	return srvRecords, nil
}

// Get - Retrieves DNS records for a bucket.
func (c *CoreDNS) Get(bucket string) ([]SrvRecord, error) {
	var srvRecords []SrvRecord
	for _, domainName := range c.domainNames {
		key := msg.Path(fmt.Sprintf("%s.%s.", bucket, domainName), c.prefixPath)
		records, err := c.list(key)
		if err != nil {
			return nil, err
		}
		// Make sure we have record.Key is empty
		// this can only happen when record.Key
		// has bucket entry with exact prefix
		// match any record.Key which do not
		// match the prefixes we skip them.
		for _, record := range records {
			if record.Key != "" {
				continue
			}
			srvRecords = append(srvRecords, record)
		}
	}
	if len(srvRecords) == 0 {
		return nil, ErrNoEntriesFound
	}
	return srvRecords, nil
}

// msgUnPath converts a etcd path to domainname.
func msgUnPath(s string) string {
	ks := strings.Split(strings.Trim(s, etcdPathSeparator), etcdPathSeparator)
	for i, j := 0, len(ks)-1; i < j; i, j = i+1, j-1 {
		ks[i], ks[j] = ks[j], ks[i]
	}
	return strings.Join(ks, ".")
}

// Retrieves list of entries under the key passed.
// Note that this method fetches entries upto only two levels deep.
func (c *CoreDNS) list(key string) ([]SrvRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
	r, err := c.etcdClient.Get(ctx, key, etcd.WithPrefix())
	defer cancel()
	if err != nil {
		return nil, err
	}
	if r.Count == 0 {
		key = strings.TrimSuffix(key, etcdPathSeparator)
		r, err = c.etcdClient.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if r.Count == 0 {
			return nil, ErrNoEntriesFound
		}
	}

	var srvRecords []SrvRecord
	for _, n := range r.Kvs {
		var srvRecord SrvRecord
		if err = json.Unmarshal([]byte(n.Value), &srvRecord); err != nil {
			return nil, err
		}
		srvRecord.Key = strings.TrimPrefix(string(n.Key), key)
		srvRecord.Key = strings.TrimSuffix(srvRecord.Key, srvRecord.Host)

		// Skip non-bucket entry like for a key
		// /skydns/net/miniocloud/10.0.0.1 that may exist as
		// dns entry for the server (rather than the bucket
		// itself).
		if srvRecord.Key == "" {
			continue
		}

		srvRecord.Key = msgUnPath(srvRecord.Key)
		srvRecords = append(srvRecords, srvRecord)

	}
	if len(srvRecords) == 0 {
		return nil, ErrNoEntriesFound
	}
	sort.Slice(srvRecords, func(i int, j int) bool {
		return srvRecords[i].Key < srvRecords[j].Key
	})
	return srvRecords, nil
}

// Put - Adds DNS entries into etcd endpoint in CoreDNS etcd message format.
func (c *CoreDNS) Put(bucket string) error {
	for ip := range c.domainIPs {
		bucketMsg, err := newCoreDNSMsg(ip, c.domainPort, defaultTTL)
		if err != nil {
			return err
		}
		for _, domainName := range c.domainNames {
			key := msg.Path(fmt.Sprintf("%s.%s", bucket, domainName), c.prefixPath)
			key = key + etcdPathSeparator + ip
			ctx, cancel := context.WithTimeout(context.Background(), defaultContextTimeout)
			_, err = c.etcdClient.Put(ctx, key, string(bucketMsg))
			defer cancel()
			if err != nil {
				ctx, cancel = context.WithTimeout(context.Background(), defaultContextTimeout)
				c.etcdClient.Delete(ctx, key)
				defer cancel()
				return err
			}
		}
	}
	return nil
}

// Delete - Removes DNS entries added in Put().
func (c *CoreDNS) Delete(bucket string) error {
	for _, domainName := range c.domainNames {
		key := msg.Path(fmt.Sprintf("%s.%s.", bucket, domainName), c.prefixPath)
		srvRecords, err := c.list(key)
		if err != nil {
			return err
		}
		for _, record := range srvRecords {
			dctx, dcancel := context.WithTimeout(context.Background(), defaultContextTimeout)
			if _, err = c.etcdClient.Delete(dctx, key+etcdPathSeparator+record.Host); err != nil {
				dcancel()
				return err
			}
			dcancel()
		}
	}
	return nil
}

// DeleteRecord - Removes a specific DNS entry
func (c *CoreDNS) DeleteRecord(record SrvRecord) error {
	for _, domainName := range c.domainNames {
		key := msg.Path(fmt.Sprintf("%s.%s.", record.Key, domainName), c.prefixPath)

		dctx, dcancel := context.WithTimeout(context.Background(), defaultContextTimeout)
		if _, err := c.etcdClient.Delete(dctx, key+etcdPathSeparator+record.Host); err != nil {
			dcancel()
			return err
		}
		dcancel()
	}
	return nil
}

// CoreDNS - represents dns config for coredns server.
type CoreDNS struct {
	domainNames []string
	domainIPs   set.StringSet
	domainPort  string
	prefixPath  string
	etcdClient  *etcd.Client
}

// Option - functional options pattern style
type Option func(*CoreDNS)

// DomainNames set a list of domain names used by this CoreDNS
// client setting, note this will fail if set to empty when
// constructor initializes.
func DomainNames(domainNames []string) Option {
	return func(args *CoreDNS) {
		args.domainNames = domainNames
	}
}

// DomainIPs set a list of custom domain IPs, note this will
// fail if set to empty when constructor initializes.
func DomainIPs(domainIPs set.StringSet) Option {
	return func(args *CoreDNS) {
		args.domainIPs = domainIPs
	}
}

// DomainPort - is a string version of server port
func DomainPort(domainPort string) Option {
	return func(args *CoreDNS) {
		args.domainPort = domainPort
	}
}

// CoreDNSPath - custom prefix on etcd to populate DNS
// service records, optional and can be empty.
// if empty then c.prefixPath is used i.e "/skydns"
func CoreDNSPath(prefix string) Option {
	return func(args *CoreDNS) {
		args.prefixPath = prefix
	}
}

// NewCoreDNS - initialize a new coreDNS set/unset values.
func NewCoreDNS(etcdClient *etcd.Client, setters ...Option) (Config, error) {
	if etcdClient == nil {
		return nil, errors.New("invalid argument")
	}

	args := &CoreDNS{
		etcdClient: etcdClient,
		prefixPath: defaultPrefixPath,
	}

	for _, setter := range setters {
		setter(args)
	}

	if len(args.domainNames) == 0 || args.domainIPs.IsEmpty() {
		return nil, errors.New("invalid argument")
	}

	// strip ports off of domainIPs
	domainIPsWithoutPorts := args.domainIPs.ApplyFunc(func(ip string) string {
		host, _, err := net.SplitHostPort(ip)
		if err != nil {
			if strings.Contains(err.Error(), "missing port in address") {
				host = ip
			}
		}
		return host
	})
	args.domainIPs = domainIPsWithoutPorts

	return args, nil
}
