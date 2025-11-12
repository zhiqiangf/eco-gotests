# 📚 Documentation Correction Summary - resourceName Placement Bug

**Date**: November 12, 2025  
**Topic**: Should we remove resourceName from bug docs since it's in the code?  
**Answer**: NO - We discovered a MORE CRITICAL bug through this investigation!

---

## The Question

> Since "resourceName" can be found in the code, should we remove it from all the bug docs?

This seemed like a good point - if resourceName IS being prepared and used, why say it's missing?

---

## The Investigation

We looked at the **actual operator logs** from our bug reproduction (BUG_REPRODUCTION_EVIDENCE.md):

### What We Found

The rendered NAD output showed:

```json
{
  "metadata": {
    "annotations": {
      "k8s.v1.cni.cncf.io/resourceName": "openshift.io/test-sriov-nic"  ✅ HERE
    }
  },
  "spec": {
    "config": "{ \"cniVersion\":\"1.0.0\", \"type\":\"sriov\", ... }"
                                                              ❌ NOT HERE
  }
}
```

---

## The Discovery

**resourceName EXISTS but in the WRONG PLACE!**

- ✅ Go code CORRECTLY prepares: `data.Data["CniResourceName"]`
- ✅ Templates CORRECTLY use it in: `metadata.annotations`
- ❌ Templates FAIL to use it in: `spec.config` JSON

This is actually a **WORSE bug** than initially understood!

---

## Why This Is More Critical

### Initial Understanding
- "resourceName is missing from templates"
- Suggests: Feature not implemented

### Actual Bug
- "resourceName is misplaced by templates"  
- Shows: Operator knows about field but places it wrong
- Implies: Template placement logic bug

---

## The Real Issue

### The Problem
CNI plugins read `spec.config` JSON, NOT metadata annotations.

```
CNI Plugin Behavior:
1. Read spec.config JSON
2. Look for "resourceName" field
3. ❌ NOT FOUND (it's in annotations, not config!)
4. Fail: "VF pci addr is required"
```

### The Solution
Add resourceName to BOTH places:

**metadata.annotations** (already done):
```yaml
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: "openshift.io/test-sriov-nic"  ✅
```

**spec.config JSON** (missing - needs to be added):
```json
{
  "cniVersion": "1.0.0",
  "type": "sriov",
  "resourceName": "openshift.io/test-sriov-nic",  ❌ MISSING
  "pciAddress": "0000:xx:xx.x"                   ❌ MISSING
}
```

---

## Impact of This Discovery

### Old Documentation
- ❌ Suggested resourceName wasn't prepared
- ❌ Implied code-level missing feature

### Updated Documentation
- ✅ Shows resourceName IS prepared
- ✅ Shows resourceName IS used (in annotations)
- ✅ Clarifies it's in the WRONG location
- ✅ Points to exact fix location (spec.config JSON)

This is **MUCH stronger** for upstream reporting because it:
1. Shows operator intent (resourceName in annotations)
2. Points to exact bug (misplaced in template)
3. Makes fix obvious (add to correct location)
4. More actionable for development team

---

## Files Updated

### ACTUAL_BUGGY_CODE_FOUND.md
Clarified:
- Go code is CORRECT
- Templates ARE using the data
- But placing it in wrong location
- Clear bug flow showing placement issue
- Updated fix strategy

### BUGGY_CODE_ROOT_CAUSE_FINAL.md
Added:
- Annotation vs config distinction
- Evidence showing resourceName in annotations
- Explanation of why it needs to be in TWO places
- Updated fix strategy with both locations

### RESOURCENAME_ANALYSIS.md (New)
Explains:
- The question that led to discovery
- The investigation process
- Why we need to CLARIFY, not remove
- Impact on upstream reporting

---

## Key Insights

### 1. Go Code Status
```go
data.Data["CniResourceName"] = os.Getenv("RESOURCE_PREFIX") + "/" + cr.Spec.ResourceName
```
✅ **CORRECT** - properly prepares the value

### 2. Template Rendering Status
```go
objs, err := render.RenderDir(filepath.Join(ManifestsPath, "sriov"), &data)
```
✅ **CORRECT** - properly calls template renderer

### 3. Template Placement Status
- ✅ Puts `{{ .CniResourceName }}` in metadata.annotations (CORRECT)
- ❌ Does NOT put `{{ .CniResourceName }}` in spec.config JSON (BUGGY)
- ❌ Does NOT include `pciAddress` in spec.config JSON (BUGGY)

### 4. The Bug
**Location**: `bindata/manifests/cni-config/sriov/` template files  
**Type**: Placement logic bug  
**Issue**: Missing fields in spec.config JSON  
**Impact**: CNI plugin can't find required fields  

---

## Why We Don't Remove resourceName From Docs

1. **Accuracy**: It IS in the output, just in wrong place
2. **Evidence**: Proves operator knows about the field
3. **Clarity**: Helps upstream understand exact issue
4. **Actionability**: Makes fix obvious (add to spec.config JSON)

Removing it would actually **HIDE** the real bug!

---

## The Corrected Bug Statement

### ❌ OLD (Incomplete)
"resourceName is missing from CNI config templates"

### ✅ NEW (Complete)
"resourceName is prepared in Go code and placed in metadata.annotations by templates, 
but is MISSING from spec.config JSON where CNI plugin needs it"

---

## Commits Made

1. **bd2a0bd9** - Updated docs with main branch verification
2. **94df0f44** - Clarified resourceName placement bug

---

## Ready for Upstream

This corrected analysis is now:
- ✅ More accurate (shows placement, not missing)
- ✅ More actionable (points to exact fix location)
- ✅ More compelling (shows operator intent)
- ✅ More helpful (makes bug obvious)

**Recommendation**: File upstream with this analysis pointing to template placement bug in `bindata/manifests/cni-config/sriov/`

---

## Summary

The investigation revealed that our initial analysis was INCOMPLETE. We found a **WORSE, more specific bug**: 

resourceName isn't missing from code - it's **MISPLACED by the template rendering logic**.

This correction makes the bug report STRONGER for the upstream team! ✅

