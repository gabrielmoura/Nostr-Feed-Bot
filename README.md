# Nostr-Feed-Bot

O Nostr-Feed-Bot é um serviço que permite publicar automaticamente feeds RSS/Atom como eventos no protocolo Nostr. Ele
busca feeds periodicamente, converte o conteúdo em Markdown e publica as entradas como notas no seu relay Nostr.

## Funcionalidades

- **Publicação Automática:** Publica automaticamente novos itens de feeds RSS/Atom no seu relay Nostr.
- **Conversão para Markdown:** Converte o conteúdo HTML dos feeds para Markdown, garantindo uma formatação limpa e
  consistente nas notas.
- **Suporte a Metadados:** Extrai e inclui metadados importantes como título, categorias, resumo, imagem, autor e data
  de publicação em cada evento.
- **Personalização:** Permite configurar quais feeds seguir e para qual relay enviar as notas.
- **Gerenciamento de Eventos:** Evita publicar eventos duplicados, utilizando links para controle.
- **API Simples:** API para adicionar e visualizar feeds.

## Como usar

### API

O serviço oferece uma API simples para gerenciar os feeds.

#### Adicionar um Feed

Para adicionar um feed, envie uma requisição POST para `/rss` com os seguintes dados no corpo da requisição como JSON:

```json
{
  "url": "URL_DO_SEU_FEED_RSS",
  "pub_key": "SUA_CHAVE_PÚBLICA_NOSTR",
  "priv_key": "SUA_CHAVE_PRIVADA_NOSTR",
  "relay": "URL_DO_SEU_RELAY_NOSTR"
}
```

-   `url`: URL do feed RSS ou Atom.
-   `pub_key`: Sua chave pública Nostr (codificada ou não).
-   `priv_key`: Sua chave privada Nostr (codificada ou não).
-   `relay`: URL do relay Nostr para onde os eventos serão publicados.

Exemplo usando `curl`:

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "url": "https://example.com/feed.xml",
  "pub_key": "npub1...",
  "priv_key": "nsec1...",
  "relay": "wss://relay.example.com"
}' http://localhost:3000/rss
```

#### Visualizar os Feeds Ativos

Para visualizar os feeds adicionados, envie uma requisição GET para `/rss`:

```bash
curl http://localhost:3000/rss
```

A resposta será um JSON contendo todos os feeds configurados.

### Configuração

-   O bot busca novos itens dos feeds a cada 2 minutos.
-   A publicação dos eventos no relay é feita a cada minuto.
-   Você pode configurar o intervalo de busca e publicação editando as strings do `cron` em `setupCron`.

### Contribuições

Sinta-se à vontade para contribuir com melhorias, correções de bugs e novas funcionalidades!
Ou enviar Sats para `verdantkite75@walletofsatoshi.com` para apoiar o desenvolvimento.

---

Feito com ❤️ para a comunidade Nostr.