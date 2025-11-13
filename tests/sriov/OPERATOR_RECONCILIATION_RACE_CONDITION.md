# ⚔️ The Operator Reconciliation Race Condition

**Visualization of the battle between our workaround and the operator**

---

## The Race Condition Timeline

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Time: 21:49:17.592                                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ SR-IOV OPERATOR                                                      │  │
│  │ "I need to create telco-mgmt-cx7anl244 NAD"                          │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ 1. Render NAD from SriovNetwork CR                                   │  │
│  │ 2. Use buggy template (missing resourceName in spec.config)          │  │
│  │ 3. Generate INCOMPLETE NAD                                           │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  Time: 21:49:17.602                                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ CREATE NAD: telco-mgmt-cx7anl244                                     │  │
│  │ Config: { "type":"sriov", ... }  ← MISSING resourceName!            │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Time: 21:49:17.596 - 21:49:19.600 (approx 2 seconds)                      │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ TEST WORKAROUND                                                      │  │
│  │ "NAD was created but is incomplete, I'll fix it!"                    │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ 1. Pull NAD from cluster                                             │  │
│  │ 2. Parse spec.config JSON                                            │  │
│  │ 3. Detect missing resourceName field                                 │  │
│  │ 4. Add resourceName: "openshift.io/cx7anl244"                        │  │
│  │ 5. Update NAD with patched config                                    │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  Time: 21:49:19.600 (approx)                                                │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ ✅ UPDATE NAD: telco-mgmt-cx7anl244                                  │  │
│  │ Config: { "type":"sriov", "resourceName":"...", ... }  ✅ COMPLETE!  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ KUBERNETES EVENT TRIGGERED                                           │  │
│  │ "NetworkAttachmentDefinition telco-mgmt-cx7anl244 was MODIFIED"      │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│                     [Event sent to operator]                                │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Time: 21:49:19.617                                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ SR-IOV OPERATOR (Reconciliation Triggered)                           │  │
│  │ "Someone modified telco-mgmt-cx7anl244 NAD!"                         │  │
│  │ "Let me check if it matches my desired state..."                     │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ 1. Re-render NAD from SriovNetwork CR                                │  │
│  │ 2. Use SAME buggy template (missing resourceName)                    │  │
│  │ 3. Generate SAME INCOMPLETE NAD                                      │  │
│  │ 4. Compare rendered NAD vs actual NAD                                │  │
│  │    - Rendered: INCOMPLETE (no resourceName in config)                │  │
│  │    - Actual:   COMPLETE (has resourceName in config)                 │  │
│  │    - Comparison: DIFFERENT!                                          │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ OPERATOR DECISION                                                    │  │
│  │ "Actual state doesn't match my desired state!"                       │  │
│  │ "I must update it to match my template!"                             │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  Time: 21:49:19.618                                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ ❌ UPDATE NAD: telco-mgmt-cx7anl244                                  │  │
│  │ Config: { "type":"sriov", ... }  ← MISSING resourceName AGAIN!      │  │
│  │                                                                      │  │
│  │ [OPERATOR OVERWRITES THE WORKAROUND PATCH]                          │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│                   NAD is INCOMPLETE again                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Time: 21:49:21.644                                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ TEST                                                                 │  │
│  │ "Let me create a pod using this NAD..."                              │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ POD CREATION                                                         │  │
│  │ Name: control-plane                                                  │  │
│  │ Networks: telco-mgmt-cx7anl244, telco-userplane-cx7anl244            │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ MULTUS CNI PLUGIN                                                    │  │
│  │ "Let me attach SR-IOV interfaces to this pod..."                     │  │
│  │ 1. Read NAD telco-mgmt-cx7anl244                                     │  │
│  │ 2. Parse spec.config                                                 │  │
│  │ 3. Look for resourceName field...                                    │  │
│  │ 4. ❌ Field not found!                                               │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ SR-IOV CNI PLUGIN                                                    │  │
│  │ "I need resourceName to identify the device!"                        │  │
│  │ ❌ ERROR: VF pci addr is required                                    │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                           ↓                                                 │
│                     Pod fails to start                                      │
│                     Retry for 10 minutes                                    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│  Time: 21:59:21.654                                                         │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ TEST TIMEOUT                                                         │  │
│  │ ❌ FAILED: context.deadlineExceededError                             │  │
│  │ Pod never became ready                                               │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## The Battle Breakdown

### Round 1: Operator Creates Incomplete NAD

**Winner**: Operator ✅  
**Result**: NAD exists but is incomplete

```
Operator: "I created the NAD according to my template."
Status: Incomplete NAD in cluster
```

---

### Round 2: Workaround Patches NAD

**Winner**: Workaround ✅  
**Result**: NAD is now complete

```
Workaround: "I detected and fixed the incomplete NAD!"
Status: Complete NAD in cluster
Duration: ~2 seconds
```

---

### Round 3: Operator Overwrites Patch

**Winner**: Operator ✅  
**Result**: NAD is incomplete again

```
Operator: "Hey, someone changed my NAD! Let me fix it back to my template."
Status: Incomplete NAD in cluster (again)
```

---

### Round 4: Pod Tries to Use NAD

**Winner**: Nobody ❌  
**Result**: Pod fails to attach

```
Pod: "I need SR-IOV interfaces..."
CNI: "This NAD is incomplete, I can't help you."
Pod: ❌ Failed
```

---

## Why Workaround Can't Win

### The Operator's Advantages

1. **Authority** - Operator is the source of truth
2. **Persistence** - Operator continuously reconciles
3. **Speed** - Operator responds to changes in ~1 second
4. **Determination** - Operator will keep overwriting forever

### The Workaround's Disadvantages

1. **One-time** - Workaround runs once during NAD creation
2. **Triggering** - Workaround update triggers operator reconciliation
3. **Temporary** - Patch lasts only until next reconciliation
4. **No authority** - Workaround can't prevent operator updates

### The Timing Problem

```
Workaround patch lifetime:
├─ Applied: 21:49:19.600
├─ Overwritten: 21:49:19.618
└─ Duration: ~18 milliseconds

Pod creation attempt:
└─ Started: 21:49:21.644 (2 seconds later)

By the time the pod tries to use the NAD, it's already incomplete again!
```

---

## What If We Try Harder?

### Scenario 1: Patch Multiple Times

```
Attempt 1: Patch NAD → Operator overwrites
Attempt 2: Patch NAD → Operator overwrites
Attempt 3: Patch NAD → Operator overwrites
...
Result: ❌ Operator always wins
```

**Why it fails:**
- Each patch triggers a new reconciliation
- Operator is faster and has priority
- Creates an endless loop

---

### Scenario 2: Patch Right Before Pod Creation

```
1. Patch NAD
2. Immediately create pod (no delay)
3. Hope pod reads NAD before operator overwrites

Result: ❌ Race condition
```

**Why it fails:**
- Unpredictable timing
- Operator might respond first
- Even if pod reads NAD, CNI might execute after overwrite
- Unreliable and fragile

---

### Scenario 3: Continuous Re-Patching

```
Background thread:
  while true:
    if NAD is incomplete:
      patch NAD
    sleep 100ms

Result: ❌ Resource intensive, still unreliable
```

**Why it fails:**
- High CPU and API server load
- Still has race conditions
- Operator can overwrite between checks
- Not sustainable

---

### Scenario 4: Remove Operator

```
1. Scale operator to 0 replicas
2. Patch NADs manually
3. Create pods
4. Scale operator back up

Result: ⚠️ Works but dangerous
```

**Why it's bad:**
- Breaks automated management
- Disables self-healing
- Defeats purpose of operator
- Not a real solution

---

## The Only Real Solution

### Fix the Operator Template

```diff
File: bindata/manifests/cni-config/sriov/network-attachment-definition.yaml

spec:
  config: |
    {
      "cniVersion": "1.0.0",
      "name": "{{ .NetworkName }}",
      "type": "sriov",
+     "resourceName": "{{ .CniResourceName }}",
      "vlan": {{ .Vlan }},
      ...
    }
```

**This changes the operator's desired state**, so:
- Operator renders complete NADs
- No overwriting needed
- Patches are unnecessary
- Everything works correctly

---

## Lessons Learned

### About Kubernetes Operators

**Operators are authoritative** - They define and enforce the desired state of resources.

**Reconciliation is continuous** - Changes to watched resources trigger immediate reconciliation.

**Manual changes are temporary** - Any manual modification will be reverted during next reconciliation.

### About Workarounds

**Resource-level workarounds fail for operator-managed resources** - You can't "outsmart" an operator by modifying its resources.

**Controller-level fixes required** - To permanently change operator behavior, you must change the operator itself.

**Tests can't workaround operator bugs** - At best, tests can detect and document operator issues.

### About This Bug

**More severe than initially assessed** - Not just incomplete generation, but active enforcement of incomplete state.

**No user-level mitigation** - Admins cannot manually fix this in production.

**Blocks SR-IOV functionality** - Prevents pod attachment, making SR-IOV unusable in affected scenarios.

---

## Summary

### The Race Condition

```
┌─────────────┐         ┌──────────────┐         ┌─────────────┐
│  Operator   │ creates │ Incomplete   │ read by │    Pod      │
│   (buggy)   │────────▶│     NAD      │────────▶│   (fails)   │
└─────────────┘         └──────────────┘         └─────────────┘
       ▲                        │
       │                        │ modifies
       │                        ▼
       │                ┌──────────────┐
       │                │  Workaround  │
       │                │   (patches)  │
       │                └──────────────┘
       │                        │
       │ overwrites             │ triggers
       │◀───────────────────────┘
       │
     [Loop repeats]
```

### The Reality

**Operator always wins** - It has authority, speed, and persistence.

**Workarounds are temporary** - They last milliseconds before being overwritten.

**Only upstream fix works** - Changing the operator's template is the only solution.

---

**Status**: Race condition fully documented and explained  
**Next**: Wait for OCPBUGS-65542 upstream fix

