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
 http://localhost:3001/api/v1/transaction/rule/333e8d7b-0997-4f00-9267-ae41622c9225 \
 -H "Content-Type: application/json" \
 -H "Accept: application/json" \
 -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
 -d @example/seller.json

 ```