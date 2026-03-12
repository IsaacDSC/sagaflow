### Create rule for use in transaction

```sh
 curl -X PUT \
 http://localhost:3001/api/v1/rule \
 -H "Content-Type: application/json" \
 -H "Accept: application/json" \
 -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
 -d @example/put_rule.json
 ```


 ### Send transaction
 ```sh
curl -X POST \
 http://localhost:3001/api/v1/transaction/rule/da844d8f-80d7-46a2-a9d5-1909df8567f3 \
 -H "Content-Type: application/json" \
 -H "Accept: application/json" \
 -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
 -d @example/seller.json

 ```