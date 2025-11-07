# SR-IOV Network Removal Failure - Sequence Diagram

## Timeline of the Failure (Lines 823-845)

```
TIME    ACTION                                          STATUS
───────────────────────────────────────────────────────────────────────────
T+0s    [Test Execution Completes]
        - Test pods created ✓
        - Network verification passed ✓
        - Ready for cleanup
        
        │
        └─→ CLEANUP PHASE STARTS
        
T+0s    STEP: Removing SRIOV network cx7anl244
        Command: oc delete sriovnetwork cx7anl244 -n openshift-sriov-network-operator
        │
        └─→ SriovNetwork CR is deleted from API server
        
T+30s   STEP: Waiting for SRIOV network to be deleted
        Poll: checkNetwork.Exists() == false
        Timeout: 30 seconds
        Status: ✓ SUCCESS (network CR deleted)
        
        │
        └─→ Now waiting for NAD to be deleted
        
T+32s   STEP: Waiting for NetworkAttachmentDefinition cx7anl244 to be deleted
        Expected: NAD should auto-delete via owner reference
        Poll Interval: 2 seconds
        Poll Timeout: 60 seconds ⚠️
        
        Expected Flow (NOT HAPPENING):
        ┌─────────────────────────────────────────────┐
        │ 1. SriovNetwork deleted                     │
        │ 2. SR-IOV Operator watches for deletion     │
        │ 3. Operator deletes corresponding NAD       │
        │ 4. Test detects NAD gone within 60s         │
        │ 5. Test passes cleanup                      │
        └─────────────────────────────────────────────┘
        
        Actual Flow (WHAT'S HAPPENING):
        ┌─────────────────────────────────────────────┐
        │ 1. SriovNetwork deleted ✓                   │
        │ 2. SR-IOV Operator should delete NAD        │
        │ 3. ❌ NAD IS STILL THERE after 60s          │
        │ 4. Poll keeps checking...                   │
        │ 5. Outer timeout (180s) expires             │
        │ 6. ❌ TEST FAILS                            │
        └─────────────────────────────────────────────┘
        
T+92s   Poll timeout expires (60 seconds elapsed)
        Error: "wait.PollUntilContextTimeout" returns error
        
T+180s  ❌ OUTER TIMEOUT EXPIRES (180 seconds / 3 minutes)
        
        Error Message:
        ─────────────────────────────────────────────
        [FAILED] Timed out after 180.002s.
        Failed to wait for NetworkAttachmentDefinition cx7anl244 
        in namespace e2e-25959-cx7anl244. 
        Ensure the SRIOV policy exists and is properly configured.
        
        Error: networkattachmentdefinition object cx7anl244 
        does not exist in namespace e2e-25959-cx7anl244
        ─────────────────────────────────────────────
        
        Location: /root/eco-gotests/tests/sriov/helpers.go:516
        Location: /root/eco-gotests/tests/sriov/sriov_basic_test.go:178
        
        ❌ TEST MARKED AS FAILED
```

---

## Component Interaction Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                    TEST EXECUTION                            │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Test Pod                  Test Pod                          │
│  ┌─────────┐              ┌─────────┐                       │
│  │ Client  │──NET1────────│ Server  │                       │
│  └─────────┘   (eth0)     └─────────┘                       │
│       │                        │                             │
│       └────────────────────────┘                             │
│                  ✓ Ping works                                │
│                                                              │
└──────────────────────────────────────────────────────────────┘
                           │
                           ▼
         ┌──────────────────────────────────────────┐
         │      CLEANUP PHASE (WHERE IT FAILS)      │
         ├──────────────────────────────────────────┤
         │                                          │
         │  rmSriovNetwork()                        │
         │  ├─ Delete SriovNetwork CR ✓            │
         │  ├─ Wait for SriovNetwork deleted ✓     │
         │  └─ Wait for NAD deleted ❌             │
         │     └─ Timeout after 180 seconds        │
         │                                          │
         └──────────────────────────────────────────┘
                           │
                           ▼
         ┌──────────────────────────────────────────┐
         │   SR-IOV OPERATOR (NOT RESPONDING)       │
         ├──────────────────────────────────────────┤
         │                                          │
         │  Operator Pod                            │
         │  ├─ Should watch SriovNetwork deletion   │
         │  ├─ Should delete NAD when SR-IOV deleted
         │  └─ ❌ NOT DELETING NAD                  │
         │                                          │
         │  Possible Issues:                        │
         │  • Operator crash/restart                │
         │  • Webhook not responding                │
         │  • RBAC permissions missing              │
         │  • Finalizers blocking cleanup           │
         │  • Controller busy or hung               │
         │                                          │
         └──────────────────────────────────────────┘


RESOURCE OWNERSHIP CHAIN (Should Enable Cascading Delete):

┌─────────────────────────────────┐
│    SriovNetwork CR              │
│  (openshift-sriov-operator NS)  │
│                                 │
│  metadata:                      │
│    name: cx7anl244              │
│                                 │
└─────────────────────────────────┘
           │ owns
           ▼
┌─────────────────────────────────┐
│  NetworkAttachmentDefinition    │
│  (test namespace)               │
│                                 │
│  metadata:                      │
│    name: cx7anl244              │
│    ownerReferences:             │
│    - kind: SriovNetwork         │
│      name: cx7anl244            │
│      uid: <...>                 │
│                                 │
│  When owner deleted:            │
│  → NAD should auto-delete ✓     │
│  (cascading deletion)           │
│                                 │
└─────────────────────────────────┘
```

---

## Code Path to Failure

```
sriov_basic_test.go:178
  │
  └─→ Test runs (passes)
  │
  └─→ Cleanup triggered
      │
      └─→ rmSriovNetwork(name, sriovOpNs)  [helpers.go:520]
          │
          ├─ Find and delete SriovNetwork  [line 562-567]
          │   status: ✓ SUCCESS
          │
          ├─ Wait for SriovNetwork deleted  [line 569-581]
          │   status: ✓ SUCCESS
          │
          ├─ Is targetNamespace != sriovOpNs?  [line 584]
          │   (test namespace != operator namespace)
          │   status: YES, proceed
          │
          └─→ Wait for NAD deleted  [line 586-600]
              │
              ├─ wait.PollUntilContextTimeout(
              │     ctx,
              │     2*time.Second,      (poll every 2 seconds)
              │     1*time.Minute,      (timeout after 60 seconds) ⚠️
              │     true,
              │     func() {...}
              │  )
              │
              ├─ T+2s:  Check NAD exists? → YES, keep waiting
              ├─ T+4s:  Check NAD exists? → YES, keep waiting
              ├─ ...
              ├─ T+92s: Poll timeout expires, returns error
              │
              └─→ if err != nil {  [line 601]
                    │
                    └─→ Expect(err).ToNot(HaveOccurred())  [line 607]
                        │
                        └─→ ❌ ASSERTION FAILS
                            Error logged at line 516 (Eventually timeout)
                            Test FAILED at sriov_basic_test.go:178
```

---

## Timeout Nesting Issue

```
OUTER EVENTUALLY TIMEOUT (180 seconds)
│
├─ INNER POLL TIMEOUT (60 seconds) ← Line 589 of helpers.go
│  ├─ Poll every 2 seconds
│  ├─ Timeout after 60 seconds
│  ├─ Expected: NAD deleted OR timeout error
│  └─ Actual: Returns error after 60s
│
├─ ERROR HANDLING (line 601-609)
│  └─ Pass error to Expect() assertion
│
└─ EVENTUALLY TIMEOUT (line 516)
   ├─ Had 180 seconds
   ├─ Used 180 seconds
   └─ ❌ TEST FAILS

The problem is NOT the 60s inner timeout.
The problem is the NAD is STILL NOT DELETED after outer timeout too!
```

---

## What Should Happen vs What Actually Happens

```
EXPECTED FLOW (SUCCESS):
─────────────────────────────────────────────

T+0s   Delete SriovNetwork
T+5s   ← Operator watches and detects SriovNetwork deletion
T+10s  ← Operator deletes associated NAD
T+15s  ← Test poll detects NAD is gone
T+20s  ✓ Test cleanup passes
       ✓ Test completes successfully

───────────────────────────────────────────────────────────────

ACTUAL FLOW (FAILURE):
─────────────────────────────────────────────

T+0s   Delete SriovNetwork
T+5s   ← Operator SHOULD delete NAD but doesn't ❌
       
       [Possible reasons]:
       • Operator not running
       • Operator crashed/restarting
       • Webhook not responding
       • NAD has finalizers (blocked)
       • Operator RBAC issue
       • Controller in error loop
       
T+32s  ← Test starts polling for NAD deletion
       (Expected: NAD already gone)
       (Actual: NAD still there ❌)

T+34s  ← Operator STILL hasn't deleted NAD ❌

... polling continues ...

T+92s  ← Inner poll timeout (60s) expires with error

T+100s ← Test re-checks: NAD still there ❌

... outer eventually keeps checking ...

T+180s ← OUTER TIMEOUT EXPIRES (3 minutes)
       ❌ TEST FAILS
       
Error: "networkattachmentdefinition object cx7anl244 
        does not exist in namespace e2e-25959-cx7anl244"
        
(Note: NAD doesn't exist is what we wanted,
 but timeout is the failure condition)
```

---

## Recovery Scenarios

```
SCENARIO 1: Operator Recovers Quickly
──────────────────────────────────────
If operator deletes NAD within 60 seconds:
  • Inner poll succeeds ✓
  • No error returned
  • Expect() assertion passes ✓
  • Test cleanup succeeds ✓

SCENARIO 2: Operator Slow but Recovers Within 180s
──────────────────────────────────────────────────
If operator is slow but deletes NAD before 180s:
  • Inner poll times out (60s)
  • Returns error
  • Expect() assertion FAILS ❌
  • Even though NAD will be deleted later
  
  FIX: Increase inner timeout to account for slow operator

SCENARIO 3: Operator Never Deletes NAD (Current Status)
────────────────────────────────────────────────────────
  • Operator is broken/not responding
  • NAD never gets deleted
  • 60s inner timeout expires with error
  • 180s outer timeout expires
  • Test FAILS ❌
  
  FIX: Fix operator AND increase timeouts as safety net
```

---

## Debugging Checklist

```
❓ Is NAD still present?
   Command: oc get net-attach-def cx7anl244 -n e2e-25959-cx7anl244
   
❓ Is SriovNetwork still present?
   Command: oc get sriovnetwork cx7anl244 -n openshift-sriov-network-operator
   
❓ Is operator running?
   Command: oc get pods -n openshift-sriov-network-operator
   
❓ Any finalizers blocking deletion?
   Command: oc get net-attach-def ... -o jsonpath='{.metadata.finalizers}'
   
❓ What do operator logs say?
   Command: oc logs -n openshift-sriov-network-operator -l app=sriov-network-operator --tail=100
   
❓ Any errors in events?
   Command: oc get events -n openshift-sriov-network-operator --sort-by='.lastTimestamp'
```

---

## Bottom Line

**The test is FAILING because:**
- SR-IOV operator is NOT automatically deleting the NetworkAttachmentDefinition
- This causes the cleanup phase to timeout
- Even after 180 seconds, the NAD is still not deleted

**Most likely cause:**
- SR-IOV operator pod is not running properly
- OR operator webhook is not responding
- OR there's a finalizer blocking the deletion
- OR operator RBAC permissions are insufficient

**See `QUICK_DEBUG_COMMANDS.md` for diagnostic commands**

