# k8s-eda — Event-Driven Architecture on Kubernetes

A step-by-step tutorial for building an event-driven system on Kubernetes using **Go**, using a **share trading platform** as the domain.

Each chapter maps to a branch so you can check out the repo at any point in the journey.

---

## The Domain — Share Trading

The system models a simplified share trading platform where users can **buy and sell shares in assets** (e.g. stocks, ETFs). The core workflow is:

```
User places a trade
  → Trade is validated and published as an event
    → Order service picks up the event and submits it to a provider
      → Provider fulfils (or rejects) the order
        → Result is published back as an event
          → User's portfolio is updated
```

### Key Concepts

| Term | Meaning |
|---|---|
| **Trade** | An instruction from a user to buy or sell a quantity of an asset at a given price |
| **Order** | A trade that has been submitted to an external provider for fulfilment |
| **Provider** | An external service (stubbed in this tutorial) that executes orders against a market |
| **Fulfilment** | Confirmation from the provider that an order has been fully or partially executed |
| **Portfolio** | A user's current holdings — updated as trades are fulfilled |

---

## Chapter Schedule

| Chapter | Branch | What You Build |
|---|---|---|
| 1 | `01-go-basics` | Core Go concepts: structs, interfaces, error handling, and a `Trade` domain model |
| 2 | `02-http-api` | A simple HTTP server exposing `POST /trades` and `GET /trades/:id` |
| 3 | `03-persistence` | In-memory store swapped for a real store; repository pattern introduced |
| 4 | `04-provider-stub` | A stub provider service that accepts orders and returns a fulfilment |
| 5 | `05-events` | Introduce a message broker; services communicate via events instead of direct calls |
| 6 | `06-multi-service` | Split into `trade-api`, `order-service`, and `portfolio-service` |
| 7 | `07-containerise` | Dockerfiles for each service; images built and run locally |
| 8 | `08-kubernetes` | Deploy everything to a local Kubernetes cluster with plain manifests |
| 9 | `09-resilience` | Retries, dead-letter queues, and graceful degradation when the provider is slow |
| 10 | `10-observability` | Structured logging, distributed tracing, and end-to-end event visibility |

---

## Chapter 1 — Go Basics (`01-go-basics`)

> **Goal:** Get comfortable with the Go concepts we'll use throughout the tutorial before adding any infrastructure.

You will write a small self-contained Go program (no frameworks, no HTTP yet) that models the core trading domain.

### Concepts covered

- Structs and field tags
- Interfaces and polymorphism
- Error handling (`error` as a value)
- Enums via `iota`
- Simple in-memory operations

### What you build

A `Trade` struct with a `Side` enum (`BUY` / `SELL`), a `TradeStore` interface, and an in-memory implementation. A short `main.go` exercises placing trades and printing their state — proving the model works before we bolt on HTTP or a broker.

```
k8s-eda/
└── main.go          ← entry point, exercises the domain
└── trade/
    ├── trade.go     ← Trade struct, Side enum, TradeStatus enum
    └── store.go     ← TradeStore interface + in-memory implementation
```

---

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) with Kubernetes enabled, or [kind](https://kind.sigs.k8s.io/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/)

---

## Checking Out a Chapter

Each chapter branch contains the completed state of that chapter. To follow along:

```bash
# Start from scratch at chapter 1
git checkout 01-go-basics

# Jump to a specific chapter
git checkout 05-events

# See all chapter branches
git branch -a | grep -E '^  [0-9]+'
```

---

## Status

Active development — chapters are being built out sequentially. See the branch list for what is currently available.

## License

MIT
