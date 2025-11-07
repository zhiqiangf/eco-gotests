# Pre-Test Cleanup Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     Test Suite Execution                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌────────────────────┐
                    │  BeforeSuite Hook  │
                    │ (Ginkgo lifecycle) │
                    └────────────────────┘
                              │
                ┌─────────────┴──────────────┐
                │                            │
                ▼                            ▼
    ┌──────────────────────────┐   ┌─────────────────────┐
    │ cleanupLeftoverResources │   │ Create test setup   │
    │     (NEW FEATURE!)       │   │ - Namespace         │
    └──────────────────────────┘   │ - Verify operator   │
                │                  │ - Pull images       │
                │                  └─────────────────────┘
    ┌───────────┴────────────┐
    │                        │
    ▼                        ▼
┌──────────────────┐  ┌──────────────────────┐
│ Clean Namespaces │  │ Clean SR-IOV Networks│
│                  │  │                      │
│ Pattern: e2e-*   │  │ Pattern: \d+-.*      │
│ Timeout: 120s    │  │ Location: sriov ns   │
│ Fallback: force  │  │                      │
└──────────────────┘  └──────────────────────┘
    │                        │
    └────────────┬───────────┘
                 │
                 ▼
    ┌──────────────────────────┐
    │ All cleanup complete     │
    │ Resources freed          │
    │ Ready for fresh tests    │
    └──────────────────────────┘
                 │
                 ▼
    ┌──────────────────────────┐
    │    Run All Tests         │
    │   (9 test cases × 6+ devices)
    └──────────────────────────┘
```

## Cleanup Function Flow

```
cleanupLeftoverResources()
│
├─► STEP 1: Clean Test Namespaces
│   │
│   └─► List all namespaces
│       │
│       ├─► Find: namespace name starts with "e2e-"?
│       │   │
│       │   ├─► YES → Attempt graceful delete
│       │   │   │
│       │   │   ├─► Wait 120 seconds for completion
│       │   │   │
│       │   │   ├─► Success?
│       │   │   │   ├─ YES → Log success, continue
│       │   │   │   └─ NO → Attempt force delete
│       │   │   │       ├─ Success? → Log, continue
│       │   │   │       └─ Fail? → Log error, continue
│       │   │
│       │   └─► NO → Skip namespace
│       │
│       └─► Move to next namespace
│
├─► STEP 2: Clean SR-IOV Networks
│   │
│   └─► List all SR-IOV networks in operator namespace
│       │
│       ├─► Find: network name matches pattern?
│       │   └─► Contains "-" AND starts with "2" or "7"?
│       │       │
│       │       ├─► YES → Delete network
│       │       │   │
│       │       │   └─► Success?
│       │       │       ├─ YES → Log success, continue
│       │       │       └─ NO → Log error, continue
│       │       │
│       │       └─► NO → Skip network
│       │
│       └─► Move to next network
│
└─► STEP 3: Log completion
    └─► "Cleanup of leftover resources completed"
```

## Resource Cleanup Cascade

When a test namespace is deleted, this cascade happens:

```
Delete Namespace "e2e-25959-cx7anl244"
    │
    ├─► Kubernetes deletes namespace
    │   │
    │   ├─► Orphan pods in namespace
    │   │   └─► Pod termination
    │   │       ├─► VF NIC detached from pod
    │   │       ├─► VF returned to pool
    │   │       └─► Resource deallocated
    │   │
    │   ├─► Delete NetworkAttachmentDefinition
    │   │   └─► NAD removed from namespace
    │   │
    │   └─► Final namespace deletion
    │
    └─► VF Resources Freed!
        ├─► Status: Available on worker node
        └─► Can be used by next test
```

## Before vs After Timeline

### Before: Manual Cleanup Needed
```
┌──────────────┐
│ Run Tests    │
└──────────────┘
       │
       ├─ Test 1: ✅ Pass
       ├─ Test 2: ✅ Pass
       │
       └─ User: Ctrl+C (interrupt)
          └─ Leaves behind:
             ├─ e2e-25959-cx7anl244
             ├─ e2e-70821-cx7anl244
             ├─ 25959-cx7anl244 network
             └─ 70821-cx7anl244 network

⏳ User manually cleans up (2-3 minutes)
   oc delete ns e2e-25959-cx7anl244
   oc delete ns e2e-70821-cx7anl244
   oc delete sriovnetwork 25959-cx7anl244 ...
   oc delete sriovnetwork 70821-cx7anl244 ...
   
   (wait for cleanup)

┌──────────────┐
│ Run Tests    │
│ (Again!)     │
└──────────────┘
       └─ Finally works ✅
```

### After: Automatic Cleanup
```
┌──────────────┐
│ Run Tests    │
└──────────────┘
       │
       ├─ BeforeSuite: cleanupLeftoverResources()
       │  └─ Find & delete leftover namespaces
       │  └─ Find & delete leftover networks
       │  └─ ✅ Clean state
       │
       ├─ Test 1: ✅ Pass
       ├─ Test 2: ✅ Pass
       │
       └─ User: Ctrl+C (interrupt)
          └─ Leaves behind resources

⏱️  Automatic! Next run:

┌──────────────┐
│ Run Tests    │
│ (Again!)     │
└──────────────┘
       │
       ├─ BeforeSuite: cleanupLeftoverResources()
       │  └─ Find e2e-25959-cx7anl244
       │  └─ Find e2e-70821-cx7anl244
       │  └─ Find 25959-cx7anl244 network
       │  └─ Find 70821-cx7anl244 network
       │  └─ Delete all (120s)
       │  └─ ✅ Clean state
       │
       └─ Tests run successfully ✅
```

## Timeout Strategy

```
Namespace Deletion Timeout Cascade:

Delete Namespace
    │
    ├─► Try graceful delete (120 seconds)
    │   │
    │   ├─► Success? ✅
    │   │   └─► Continue
    │   │
    │   └─► Timeout/Error? ❌
    │       │
    │       └─► Try force delete
    │           │
    │           ├─► Success? ✅
    │           │   └─► Continue (may leave dangling finalizers)
    │           │
    │           └─► Fail? ❌
    │               └─► Log error, continue
    │                   (namespace stuck, may need manual intervention)
    │
    └─► Result: Either deleted or logged for manual cleanup
```

## State Diagram

```
                START
                  │
                  ▼
          ┌─────────────────┐
          │  List Resources │
          │                 │
          │ Namespaces ✓    │
          │ Networks ✓      │
          └────────┬────────┘
                   │
         ┌─────────┴──────────┐
         │                    │
         ▼                    ▼
    ┌─────────┐          ┌──────────┐
    │ Clean   │          │ Clean    │
    │ NS Loop │          │ Net Loop │
    └────┬────┘          └─────┬────┘
         │                     │
    ┌────▼─────┐          ┌────▼──────┐
    │ More NS? │◄──NO──┐   │ More Net? │
    │          │       │   │          │
    │ YES ──┐  │       └──►│ YES ──┐  │
    └───────┼──┘    ┌──────┴───┼────┘
            │       │          │
            └───────┼──┐       │
                    │  │       │
                    ▼  │       │
                ┌─────┐ │   ┌───▼────┐
                │ END ◄─┴───┤ Return │
                │     │     │        │
                └─────┘     └────────┘
```

## Resource Tracking

```
Kubernetes Cluster
│
├─► Namespace Layer
│   ├─ namespace: e2e-25959-cx7anl244
│   │  └─► Contains:
│   │      ├─ Pod: client (using VF)
│   │      ├─ Pod: server (using VF)
│   │      └─ NAD: 25959-cx7anl244
│   │
│   └─ namespace: e2e-70821-cx7anl244
│      └─► Contains: pods + NAD
│
├─► SR-IOV Operator Namespace
│   ├─ sriovnetwork: 25959-cx7anl244
│   │  └─ Status: synced
│   │
│   └─ sriovnetwork: 70821-cx7anl244
│      └─ Status: synced
│
└─► Worker Nodes
    └─ Node: wsfd-advnetlab244
       └─► Resource: openshift.io/cx7anl244
           ├─ Capacity: 2 VFs
           ├─ Allocatable: 0 VFs (both in use!)
           │  ├─ Pod client: using 1 VF
           │  └─ Pod server: using 1 VF
           │
           Cleanup removes these pods
           └─ Result: Allocatable: 2 VFs ✅
```

## Log Output Example

```
[sig-networking] SDN sriov-legacy
  STEP: BeforeSuite [setup]
  STEP: Cleaning up leftover resources from previous test runs
  STEP: Cleaning up leftover test namespaces from previous runs
  "level"=0 "msg"="Removing leftover test namespace" "namespace"="e2e-25959-cx7anl244"
  "level"=0 "msg"="Removing leftover test namespace" "namespace"="e2e-70821-cx7anl244"
  STEP: Cleaning up leftover SR-IOV networks from previous runs
  "level"=0 "msg"="Removing leftover SR-IOV network" "network"="25959-cx7anl244"
  "level"=0 "msg"="Removing leftover SR-IOV network" "network"="70821-cx7anl244"
  "level"=0 "msg"="Cleanup of leftover resources completed"
  STEP: Creating test namespace with privileged labels
  STEP: Creating namespace sriov-basic-test
  STEP: Verifying if sriov tests can be executed on given cluster
  STEP: Pulling test images on cluster before running test cases
  
  [PASSED] BeforeSuite [setup]
  
  [PASSED] Test 25959: SR-IOV VF with spoof checking enabled
  [PASSED] Test 70821: SR-IOV VF with trust enabled
  ...
```

## Cleanup Success Indicators

```
✅ CLEANUP SUCCESSFUL when:
   │
   ├─► Log line: "Cleanup of leftover resources completed"
   │
   ├─► No error messages like:
   │   ├─ "Failed to delete leftover namespace"
   │   ├─ "Failed to delete leftover SR-IOV network"
   │   └─ "Failed to list namespaces for cleanup"
   │
   └─► Tests proceed to normal execution

⚠️  PARTIAL CLEANUP when:
   │
   ├─► Some resources deleted successfully
   ├─► Some resources failed to delete
   │
   └─► Tests may fail with resource errors
       (Need to investigate failed deletions in log)

❌ CLEANUP FAILED when:
   │
   ├─► Tests immediately fail with "Insufficient resources"
   │
   └─► Need to:
       ├─ Check cleanup log output for errors
       ├─ Manually delete stuck resources
       └─ Retry test run
```

