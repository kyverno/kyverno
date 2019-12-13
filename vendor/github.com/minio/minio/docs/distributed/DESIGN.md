# Distributed Server Design Guide [![Slack](https://slack.min.io/slack?type=svg)](https://slack.min.io)
This document explains the design approach and advanced use cases of the MinIO distributed server.

## Command-line
```
NAME:
  minio server - start object storage server.

USAGE:
  minio server [FLAGS] DIR1 [DIR2..]
  minio server [FLAGS] DIR{1...64}

DIR:
  DIR points to a directory on a filesystem. When you want to combine
  multiple drives into a single large system, pass one directory per
  filesystem separated by space. You may also use a '...' convention
  to abbreviate the directory arguments. Remote directories in a
  distributed setup are encoded as HTTP(s) URIs.
```

## Common usage

Standalone erasure coded configuration with 4 sets with 16 disks each.
```
minio server dir{1...64}
```

Distributed erasure coded configuration with 64 sets with 16 disks each.

```
minio server http://host{1...16}/export{1...64}
```

## Architecture

Expansion of ellipses and choice of erasure sets based on this expansion is an automated process in MinIO. Here are some of the details of our underlying erasure coding behavior.

- Erasure coding used by MinIO is [Reed-Solomon](https://github.com/klauspost/reedsolomon) erasure coding scheme, which has a total shard maximum of 256 i.e 128 data and 128 parity. MinIO design goes beyond this limitation by doing some practical architecture choices.

- Erasure set is a single erasure coding unit within a MinIO deployment. An object is sharded within an erasure set. Erasure set size is automatically calculated based on the number of disks. MinIO supports unlimited number of disks but each erasure set can be upto 16 disks and a minimum of 4 disks.

- We limited the number of drives to 16 for erasure set because, erasure code shards more than 16 can become chatty and do not have any performance advantages. Additionally since 16 drive erasure set gives you tolerance of 8 disks per object by default which is plenty in any practical scenario.

- Choice of erasure set size is automatic based on the number of disks available, let's say for example if there are 32 servers and 32 disks which is a total of 1024 disks. In this scenario 16 becomes the erasure set size. This is decided based on the greatest common divisor (GCD) of acceptable erasure set sizes ranging from *4, 6, 8, 10, 12, 14, 16*.

- *If total disks has many common divisors the algorithm chooses the minimum amounts of erasure sets possible for a erasure set size of any N*.  In the example with 1024 disks - 4, 8, 16 are GCD factors. With 16 disks we get a total of 64 possible sets, with 8 disks we get a total of 128 possible sets, with 4 disks we get a total of 256 possible sets. So algorithm automatically chooses 64 sets, which is *16 * 64 = 1024* disks in total.

- In this algorithm, we also make sure that we spread the disks out evenly. MinIO server expands ellipses passed as arguments. Here is a sample expansion to demonstrate the process.

```
minio server http://host{1...4}/export{1...8}
```

Expected expansion
```
> http://host1/export1
> http://host2/export1
> http://host3/export1
> http://host4/export1
> http://host1/export2
> http://host2/export2
> http://host3/export2
> http://host4/export2
> http://host1/export3
> http://host2/export3
> http://host3/export3
> http://host4/export3
> http://host1/export4
> http://host2/export4
> http://host3/export4
> http://host4/export4
> http://host1/export5
> http://host2/export5
> http://host3/export5
> http://host4/export5
> http://host1/export6
> http://host2/export6
> http://host3/export6
> http://host4/export6
> http://host1/export7
> http://host2/export7
> http://host3/export7
> http://host4/export7
> http://host1/export8
> http://host2/export8
> http://host3/export8
> http://host4/export8
```

A noticeable trait of this expansion is that it chooses unique hosts such that the erasure code is efficient across drives and hosts.

- Choosing an erasure set for the object is decided during `PutObject()`, object names are used to find the right erasure set using the following pseudo code.
```go
// hashes the key returning an integer.
func crcHashMod(key string, cardinality int) int {
        keyCrc := crc32.Checksum([]byte(key), crc32.IEEETable)
        return int(keyCrc % uint32(cardinality))
}
```
Input for the key is the object name specified in `PutObject()`, returns a unique index. This index is one of the erasure sets where the object will reside. This function is a consistent hash for a given object name i.e for a given object name the index returned is always the same.

- Write and Read quorum are required to be satisfied only across the erasure set for an object. Healing is also done per object within the erasure set which contains the object.

- MinIO does erasure coding at the object level not at the volume level, unlike other object storage vendors. This allows applications to choose different storage class by setting `x-amz-storage-class=STANDARD/REDUCED_REDUNDANCY` for each object uploads so effectively utilizing the capacity of the cluster. Additionally these can also be enforced using IAM policies to make sure the client uploads with correct HTTP headers.

## Other usages

### Advanced use cases with multiple ellipses

Standalone erasure coded configuration with 4 sets with 16 disks each, which spawns disks across controllers.
```
minio server /mnt/controller{1...4}/data{1...16}
```

Standalone erasure coded configuration with 16 sets, 16 disks per set, across mounts and controllers.
```
minio server /mnt{1..4}/controller{1...4}/data{1...16}
```

Distributed erasure coded configuration with 2 sets, 16 disks per set across hosts.
```
minio server http://host{1...32}/disk1
```

Distributed erasure coded configuration with rack level redundancy 32 sets in total, 16 disks per set.
```
minio server http://rack{1...4}-host{1...8}.example.net/export{1...16}
```

## Backend `format.json` changes

`format.json` has new fields

- `disk` is changed to `this`
- `jbod` is changed to `sets` , along with this change sets is also a two dimensional list representing total sets and disks per set.

A sample `format.json` looks like below

```json
{
  "version": "1",
  "format": "xl",
  "xl": {
    "version": "2",
    "this": "4ec63786-3dbd-4a9e-96f5-535f6e850fb1",
    "sets": [
    [
      "4ec63786-3dbd-4a9e-96f5-535f6e850fb1",
      "1f3cf889-bc90-44ca-be2a-732b53be6c9d",
      "4b23eede-1846-482c-b96f-bfb647f058d3",
      "e1f17302-a850-419d-8cdb-a9f884a63c92"
    ], [
      "2ca4c5c1-dccb-4198-a840-309fea3b5449",
      "6d1e666e-a22c-4db4-a038-2545c2ccb6d5",
      "d4fa35ab-710f-4423-a7c2-e1ca33124df0",
      "88c65e8b-00cb-4037-a801-2549119c9a33"
       ]
    ],
    "distributionAlgo": "CRCMOD"
  }
}
```

New `format-xl.go` behavior is format structure is used as a opaque type, `Format` field signifies the format of the backend. Once the format has been identified it is now the job of the identified backend to further interpret the next structures and validate them.

```go
type formatType string

const (
     formatFS formatType = "fs"
     formatXL            = "xl"
)

type format struct {
     Version string
     Format  BackendFormat
}
```

### Current format

```go
type formatXLV1 struct{
     format
     XL struct{
        Version string
        Disk string
        JBOD []string
     }
}
```

### New format

```go
type formatXLV2 struct {
        Version string `json:"version"`
        Format  string `json:"format"`
        XL      struct {
                Version          string     `json:"version"`
                This             string     `json:"this"`
                Sets             [][]string `json:"sets"`
                DistributionAlgo string     `json:"distributionAlgo"`
        } `json:"xl"`
}
```
