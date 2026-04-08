### Create rule for use in transaction

```sh
 curl -X PUT \
 http://localhost:3001/api/v1/rule \
 -H "Content-Type: application/json" \
 -H "Accept: application/json" \
 -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
 -d @example/put_rule.json
 ```

 ```sh
 curl -X PUT \
 http://localhost:3001/api/v1/rule \
 -H "Content-Type: application/json" \
 -H "Accept: application/json" \
 -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
 -d @example/put_rule_non_parallel.json
 ```

<!-- 1c85c903-145e-40b5-8de8-6ea9608ae68f parallel-->
<!-- 08f74ce1-9751-4e4e-b6b9-c55c640726d1  non-parallel-->
 ### Send transaction
 ```sh
curl -X POST \
 http://localhost:3001/api/v1/transaction/rule/1c85c903-145e-40b5-8de8-6ea9608ae68f \
 -H "Content-Type: application/json" \
 -H "Accept: application/json" \
 -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
 -d @example/seller.json

 ```