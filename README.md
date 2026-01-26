# Inference Cost Attribution Engine (ICAE)

Audit-grade cost attribution for AI intelligence execution, making every unit of cost economically legible and traceable.

## Overview

ICAE is a deterministic, auditable system that attributes costs to individual actions in AI inference workflows. It serves as the ground-truth ledger for intelligence execution costs, answering precisely "how much did this intelligence cost, and why?"

ICAE operates on the principle that if a cost cannot be attributed, it is treated as a system failure. The system produces explicit, itemized cost events that are versioned, replayable, and aggregatable without loss of provenance.

## Architecture

<pre>
┌─────────────────┐    ┌──────────────────┐    ┌──────────────────────┐
│   External      │    │  Cost Adapters   │    │   Pricing Models     │
│   Systems       │───▶│                  │───▶│                      │
│  (DIO, ZT-AAS)  │    │  - Execution     │    │  - Versioned         │
│                 │    │    Transcript    │    │    Pricing Data      │
└─────────────────┘    │  - Tool Logs     │    │  - Tiered Pricing    │
                       │  - API Metadata  │    │  - Fixed Fees        │
                       └──────────────────┘    └──────────────────────┘
                                 │                        │ 
                                 ▼                        ▼ 
                       ┌──────────────────┐    ┌──────────────────────┐
                       │   Cost Events    │    │   Cost Ledger        │
                       │                  │    │                      │
                       │  - Event ID      │    │  - Append-only       │
                       │  - Timestamp     │    │  - Tamper-evident    │
                       │  - Execution ID  │    │  - Deterministic     │
                       │  - Component     │    │  - Hashable          │
                       │  - Action        │    │  - Replayable        │
                       │  - Unit Cost     │    │                      │
                       │  - Quantity      │    └──────────────────────┘
                       │  - Total Cost    │              │ 
                       │  - Currency      │              ▼ 
                       │  - Cost Source   │    ┌──────────────────────┐
                       │  - Pricing Ver.  │    │  Replay Engine       │
                       │  - Base Unit     │    │                      │
                       │  - Metadata      │    │  - Cost Reproduction │
                       └──────────────────┘    │  - Integrity Check   │
                                               │  - Delta Analysis    │
                                               └──────────────────────┘
</pre>

## Components

### Cost Event Model  
Immutable data structure representing a single cost attribution. Enforces deterministic, explicit cost recording with full provenance metadata for auditability.

### Pricing Models  
Versioned, immutable pricing definitions supporting token-based, request-based, and time-based pricing with tiered pricing structures and fixed fees.

### Cost Ledger  
Append-only, tamper-evident storage of cost events. Deterministic ordering and hashing for verification, with per-run, per-component, and aggregate views.

### Replay Engine  
Recomputes costs using current pricing models, verifies ledger integrity through deterministic replay, and identifies discrepancies between original and replayed costs.

### Adapters  
Convert external data formats into cost events through explicit, non-invasive integration points. Supports execution transcripts, tool logs, and API metadata.

## Build

```bash
go build
```

## Test

```bash
go test ./... -v
```

## Run

```bash
./icae # Linux/macOS

.\icae.exe # Windows
```

## Design Principles

1. **Economic Legibility** - Every cost must be traceable to its source. No aggregation is allowed at the event level.
2. **Determinism and Replayability** - All cost calculations are deterministic and can be reproduced exactly given the execution transcript, pricing version snapshot, and system state.
3. **Auditability Over Convenience** - The system prioritizes transparency and verifiability over convenience features.
4. **Explicit Failure Semantics** - All failure modes are explicitly modeled and recorded as first-class events.

## Requirements

- Go 1.21+