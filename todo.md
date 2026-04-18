1. Adicionar pacote de logs slog ✅
2. Configurar timeout do servico para leitura e escrita ✅
3. Configurar gracefullshutdown ✅
4. Configurar rota de health check ✅
5. Configurar dockerfile ✅
6. Configurar docker-compose de banco mysql e postgres para exemplos de testes 
7. Configurar cli
8. Criar testes na aplicação ✅
9. Mudar config algo como
```json
"configs": {
    "max_retry": 3,
    "max_timeout": "1m",
    "kind": "" //sync  || sync-parallel || async
}

```