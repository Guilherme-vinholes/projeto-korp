# Release Notes — Projeto Korp
**Versão:** 1.0.0  
**Data:** 22/06/2026  
**Autor:** Guilherme Vinholes  
**Repositório:** https://github.com/Guilherme-vinholes/projeto-korp

---

## Sumário

- [Visão Geral](#visão-geral)
- [Arquitetura](#arquitetura)
- [Parte 1 — Serviço e Infraestrutura](#parte-1--serviço-e-infraestrutura)
- [Parte 2 — Observabilidade](#parte-2--observabilidade)
- [Parte 3 — Automação com Ansible](#parte-3--automação-com-ansible)
- [GitFlow e Estratégia de Branching](#gitflow-e-estratégia-de-branching)
- [Decisões Técnicas](#decisões-técnicas)
- [Como Executar](#como-executar)

---

## Visão Geral

Este projeto implementa um serviço HTTP em Golang com infraestrutura completa em containers Docker, monitoramento com Prometheus e Grafana, e automação de provisionamento via Ansible. O desenvolvimento seguiu boas práticas de GitFlow, com branches de feature isoladas, revisão e merge controlado até a branch `main`.

---

## Arquitetura

```
                    ┌─────────────────────────────────────────┐
                    │           Docker Network: korp-net       │
                    │                 (bridge)                 │
                    │                                          │
  HTTP :80          │   ┌──────────┐       ┌───────────────┐  │
 ─────────────────► │   │  nginx   │──────►│  go-server    │  │
                    │   │ :80      │ proxy │  :8080        │  │
                    │   └──────────┘       └───────┬───────┘  │
                    │                              │ /metrics  │
                    │   ┌──────────┐               │           │
  HTTP :9090        │   │Prometheus│◄──────────────┘           │
 ─────────────────► │   │ :9090    │                           │
                    │   └────┬─────┘                           │
                    │        │ datasource                      │
  HTTP :3000        │   ┌────▼─────┐                           │
 ─────────────────► │   │ Grafana  │                           │
                    │   │ :3000    │                           │
                    │   └──────────┘                           │
                    └─────────────────────────────────────────┘
```

---

## Parte 1 — Serviço e Infraestrutura

### Serviço HTTP em Go

O serviço `http-server-projeto-korp` foi desenvolvido em Go utilizando apenas a biblioteca padrão (`net/http`) para o servidor HTTP, com adição do SDK oficial do Prometheus para exposição de métricas.

**Endpoint implementado:**

```
GET /projeto-korp
```

**Resposta:**
```json
{
  "nome": "Projeto Korp",
  "horario": "2026-06-22T00:34:50Z"
}
```

O campo `horario` é resolvido dinamicamente a cada requisição em UTC, utilizando `time.Now().UTC().Format(time.RFC3339)`.

### Dockerfile — Multi-Stage Build

O Dockerfile foi construído com **multi-stage build** para garantir uma imagem final mínima e segura:

- **Stage 1 (builder):** `golang:1.22-alpine` — compila o binário com `CGO_ENABLED=0` para produzir um binário estático
- **Stage 2 (runtime):** `alpine:3.20` — contém apenas o binário compilado, sem toolchain Go

Resultado: imagem final com menos de 15MB, sem dependências desnecessárias.

### Rede Docker

Criada uma rede `korp-net` no modo `bridge`, garantindo isolamento e comunicação interna entre os containers sem exposição direta ao host.

### Docker Compose

Configuração com 4 serviços:

| Serviço | Imagem | Porta Host | Observação |
|---|---|---|---|
| `go-server` | build local | — | Sem exposição ao host, acesso apenas via Nginx |
| `nginx` | nginx:1.27-alpine | 80 | Proxy reverso para go-server:8080 |
| `prometheus` | prom/prometheus:v2.53.0 | 9090 | Coleta métricas do go-server |
| `grafana` | grafana/grafana:11.1.0 | 3000 | Dashboard de observabilidade |

O `go-server` não expõe portas ao host intencionalmente — o acesso externo é feito exclusivamente via Nginx, seguindo o princípio de least privilege e isolamento de rede.

### Proxy Reverso Nginx

O arquivo `nginx/http-server-projeto-korp.conf` configura o Nginx para encaminhar todas as requisições de `localhost:80` para o serviço interno `go-server:8080` via resolução de nome DNS interno do Docker.

---

## Parte 2 — Observabilidade

### Métricas Implementadas

O serviço expõe métricas no endpoint `/metrics` no padrão Prometheus:

| Métrica | Tipo | Descrição | Labels |
|---|---|---|---|
| `service_up` | Gauge | Disponibilidade do serviço (1=UP, 0=DOWN) | — |
| `http_requests_total` | Counter | Total de requisições recebidas | `method`, `path`, `status` |
| `http_request_duration_seconds` | Histogram | Latência das requisições | `method`, `path` |

A instrumentação foi implementada via middleware (`instrumento`), que envolve cada handler e registra automaticamente as métricas de cada requisição, sem acoplamento à lógica de negócio.

### Prometheus

Configurado para coletar métricas do `go-server:8080` a cada **15 segundos**.

### Grafana — Provisionamento Automatizado

O Grafana foi configurado integralmente via **arquivos de provisionamento** (bônus do desafio), sem necessidade de configuração manual:

- `grafana/provisioning/datasources/prometheus.yml` — registra o Prometheus como datasource padrão com UID fixo
- `grafana/provisioning/dashboards/dashboards.yml` — configura o provedor de dashboards apontando para o volume montado
- `grafana/dashboards/dashboard.json` — dashboard com os seguintes painéis:
  - **Disponibilidade do Serviço** — stat panel com mapeamento UP/DOWN
  - **Volume de Requisições por Segundo** — time series com `rate(http_requests_total[1m])`
  - **Latência das Requisições (p50 / p95 / p99)** — time series com `histogram_quantile`

Ao subir o ambiente, o Grafana já inicializa com o datasource e o dashboard configurados automaticamente.

---

## Parte 3 — Automação com Ansible

O playbook `ansible/playbook.yml` automatiza o provisionamento completo do ambiente em um único comando:

```bash
ansible-playbook -i ansible/inventory.ini ansible/playbook.yml
```

### Tarefas executadas

1. **Instalação do Docker Engine**
   - Dependências do sistema (`ca-certificates`, `curl`, `gnupg`)
   - Chave GPG e repositório oficial do Docker
   - Pacotes: `docker-ce`, `docker-ce-cli`, `containerd.io`, `docker-compose-plugin`
   - Serviço Docker iniciado e habilitado no systemd

2. **Cópia do projeto**
   - Criação do diretório `/opt/projeto-korp`
   - Cópia dos arquivos via módulo `copy` com preservação de permissões

3. **Build e execução**
   - Build da imagem `http-server-projeto-korp` via módulo `community.docker.docker_image`
   - Subida de todos os containers via `community.docker.docker_compose_v2`

4. **Validação automática**
   - Aguarda o serviço responder com retries (10 tentativas, intervalo de 5s)
   - Exibe a resposta do endpoint `/projeto-korp` no console via `debug`

---

## GitFlow e Estratégia de Branching

O projeto seguiu o modelo **GitFlow** com branches de feature isoladas por funcionalidade:

```
main
└── develop
    ├── feature/PK-001-go-http-server    → Serviço Go + Dockerfile
    ├── feature/PK-002-docker-compose    → Docker Compose + Nginx + Rede
    ├── feature/PK-003-prometheus-grafana → Observabilidade completa
    └── feature/PK-004-ansible-playbook  → Automação Ansible
```

### Fluxo adotado

1. Criação de branch `feature/PK-XXX-descricao` a partir de `develop`
2. Desenvolvimento e commits na feature branch
3. Merge da feature em `develop` ao concluir
4. Merge de `develop` em `main` após validação completa do ambiente

### Padrão de commits

Os commits seguiram o padrão **Conventional Commits**:
- `feat:` para novas funcionalidades
- `fix:` para correções
- `docs:` para documentação

---

## Decisões Técnicas

| Decisão | Justificativa |
|---|---|
| Multi-stage build no Dockerfile | Imagem final mínima, sem toolchain Go, menor superfície de ataque |
| go-server sem porta exposta ao host | Isolamento de rede — acesso externo somente via Nginx |
| Middleware de instrumentação | Desacopla métricas da lógica de negócio, facilita extensão futura |
| UID fixo no datasource Grafana | Garante que o dashboard provisionado encontre o datasource correto ao inicializar |
| `restart: unless-stopped` em todos os serviços | Resiliência — containers reiniciam automaticamente em caso de falha |
| Ansible com `uri` + `retries` na validação | Aguarda o serviço estar pronto antes de validar, evitando falso negativo |

---

## Como Executar

### Pré-requisitos

- Docker Desktop (Windows/Mac) ou Docker Engine (Linux)
- Docker Compose v2

### Subir o ambiente

```bash
git clone https://github.com/Guilherme-vinholes/projeto-korp.git
cd projeto-korp
docker compose up --build
```

### Validar

```bash
curl http://localhost/projeto-korp
```

**Resposta esperada:**
```json
{"nome":"Projeto Korp","horario":"2026-06-22T00:34:50Z"}
```

### Interfaces

| Serviço | URL | Credenciais |
|---|---|---|
| Aplicação | http://localhost/projeto-korp | — |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3000 | admin / admin |

### Via Ansible (Linux)

```bash
ansible-playbook -i ansible/inventory.ini ansible/playbook.yml
```

---

*Desenvolvido por Guilherme Vinholes — Junho/2026*
