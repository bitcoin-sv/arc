# MAPI

## build interface from yaml

```shell
oapi-codegen -config config.yaml mapi.yml > mapi.go
```
## API errors coming from node

Contains
- txn-mempool-conflict
- missing inputs
- mandatory-script-verify-flag-failed
- dial tcp
- 502 bad gateway
- 503 service temporarily unavailable
- context deadline exceeded
- broadcast.return_result was not success
- the network does not appear to fully agree
- too-long-mempool-chain