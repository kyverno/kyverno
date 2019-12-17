# Keycloak Quickstart Guide [![Slack](https://slack.min.io/slack?type=svg)](https://slack.min.io)

Keycloak is an open source Identity and Access Management solution aimed at modern applications and services, this document covers configuring Keycloak to be used as an identity provider for MinIO server STS API.

## 1. Prerequisites

- JAVA 1.8 and above installed
- Download and start Keycloak server by following the [installation guide](https://www.keycloak.org/docs/latest/getting_started/index.html) (finish upto section 3.4)

## 2. Configure Keycloak
- Go to Clients -> Click on account -> Settings -> Enable `Implicit Flow`, then Save.
- Go to Users -> Click on the user -> Attribute, add a new attribute `Key` is `policy`, `Value` is name of the policy in minio (ex: `readwrite`). Click Add and then Save.
- Go to Clients -> Click on `account` -> Settings, set `Valid Redirect URIs` to `*`, expand `Advanced Settings` and set `Access Token Lifespan` to `1 Hours`, then Save.
- Go to Clients -> Client on `account` -> Mappers -> Create, `Name` can be any text, `Mapper Type` is `User Attribute`, `User Attribute` is `policy`, `Token Claim Name` is `policy`, `Claim JSON Type` is `string`, then Save.
- Open http://localhost:8080/auth/realms/demo/.well-known/openid-configuration and see if it has `authorization_endpoint` and `jwks_uri`

## 3. Configure MinIO
```
$ export MINIO_ACCESS_KEY=minio
$ export MINIO_SECRET_KEY=minio123
$ minio server /mnt/export
```

Set `identity_openid` config with `config_url`, `client_id` and restart MinIO
```
~ mc admin config set myminio identity_openid config_url="http://localhost:8080/auth/realms/demo/.well-known/openid-configuration" client_id="account"
```

Once successfully set restart the MinIO instance.
```
mc admin service restart myminio
```

## 4. Using WebIdentiy API
Client ID can be found by clicking any of the clients listed [here](http://localhost:8080/auth/admin/master/console/#/realms/demo/clients). If you have followed the above steps docs, the default Client ID will be `account`.

```
$ go run docs/sts/web-identity.go -cid account -csec 072e7f00-4289-469c-9ab2-bbe843c7f5a8  -config-ep "http://localhost:8080/auth/realms/demo/.well-known/openid-configuration" -port 8888
2018/12/26 17:49:36 listening on http://localhost:8888/
```

This will open the login page of keycloak, upon successful login, STS credentials will be printed on the screen, for example

```
##### Credentials
{
	"accessKey": "6N2BALX7ELO827DXS3GK",
	"secretKey": "23JKqAD+um8ObHqzfIh+bfqwG9V8qs9tFY6MqeFR",
	"expiration": "2019-10-01T07:22:34Z",
	"sessionToken": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJhY2Nlc3NLZXkiOiI2TjJCQUxYN0VMTzgyN0RYUzNHSyIsImFjciI6IjAiLCJhdWQiOiJhY2NvdW50IiwiYXV0aF90aW1lIjoxNTY5OTEwNTUyLCJhenAiOiJhY2NvdW50IiwiZW1haWxfdmVyaWZpZWQiOmZhbHNlLCJleHAiOjE1Njk5MTQ1NTQsImlhdCI6MTU2OTkxMDk1NCwiaXNzIjoiaHR0cDovL2xvY2FsaG9zdDo4MDgxL2F1dGgvcmVhbG1zL2RlbW8iLCJqdGkiOiJkOTk4YTBlZS01NDk2LTQ4OWYtYWJlMi00ZWE5MjJiZDlhYWYiLCJuYmYiOjAsInBvbGljeSI6InJlYWR3cml0ZSIsInByZWZlcnJlZF91c2VybmFtZSI6Im5ld3VzZXIxIiwic2Vzc2lvbl9zdGF0ZSI6IjJiYTAyYTI2LWE5MTUtNDUxNC04M2M1LWE0YjgwYjc4ZTgxNyIsInN1YiI6IjY4ZmMzODVhLTA5MjItNGQyMS04N2U5LTZkZTdhYjA3Njc2NSIsInR5cCI6IklEIn0._UG_-ZHgwdRnsp0gFdwChb7VlbPs-Gr_RNUz9EV7TggCD59qjCFAKjNrVHfOSVkKvYEMe0PvwfRKjnJl3A_mBA"
}
```

These credentials can now be used to perform MinIO API operations.

## 5. Using MinIO Browser

- Open MinIO url on the browser, for example `http://localhost:9000`
- Click on `Log in with OpenID`
- Provide `Client ID` and press ENTER
- Now the user will be redirected to the Keycloak login page, upon successful login the user will be redirected to MinIO page and logged in automatically

## Explore Further

- [MinIO STS Quickstart Guide](https://docs.min.io/docs/minio-sts-quickstart-guide)
- [The MinIO documentation website](https://docs.min.io)
