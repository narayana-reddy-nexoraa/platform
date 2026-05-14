# Analyse and Fix SOP Workflow in Dify Studio

Analyse a generated SOP workflow in the Dify studio and fix compliance gaps.

## Arguments
- $ARGUMENTS: The SOP ID to analyse (e.g., "FS-01", "INS-01", "CPR-01"). If empty, ask the user which SOP to analyse.

## Instructions

You are analysing an SOP workflow that was generated in the Nexoraa AI Studio (Dify-based) at http://34.228.19.10:3000/apps.

### Step 1: Load SOP Reference Data

Read the SOP definition from the Go codebase to know what the workflow SHOULD contain:
- Read `internal/sop/registry/` files to find the SOP by ID
- Extract: steps, HITL gates, compliance frameworks, input variables, data sources, target systems
- Also read the individual SOP document from `/Users/narayana-nexoraa/Developer/HSD/SOP/individual/{SOP_ID}.md` for the full SOP details

### Step 2: Navigate to the Workflow

Use Chrome browser tools (`/chrome`):
1. Call `tabs_context_mcp` to get current tabs
2. Create a new tab or use existing one
3. Navigate to `http://34.228.19.10:3000/apps?isCreatedByMe=true`
4. Find the workflow card matching the SOP ID
5. Click to open the workflow editor

### Step 3: Read All Workflow Nodes

Important browser automation lessons (from memory):
- Use `find` tool to locate nodes by name, then `scroll_to` with ref_id
- Use `read_page` with depth 10 to get the full node tree
- Click each node to read its settings panel
- Do NOT try to scroll the canvas with `scroll` action — use `find` + `scroll_to`

For each node, capture:
- Node name and type (LLM, Classifier, Aggregator, START, END, etc.)
- Model used (gpt-4o-mini, claude-opus-4-6, etc.)
- Input variables (for START node)
- System prompt content (for LLM nodes)
- Class definitions (for Classifier nodes)
- Connected next steps

### Step 4: Gap Analysis

Compare the workflow against the SOP definition. Check for:

**Critical (must fix):**
- [ ] Classifier has correct number of risk tiers/classes matching SOP (e.g., 4 for CPR-01: Low/Medium/High/Prohibited)
- [ ] HITL gates present where SOP Section "Controls and Governance" requires human approval
- [ ] All input variables from SOP Section "Inputs" are in the START node
- [ ] Prohibited/critical paths have mandatory escalation

**Important:**
- [ ] All SOP procedure steps (Section 6) are represented as nodes
- [ ] Compliance frameworks are referenced in prompts (HIPAA, BSA/AML, 21 CFR Part 11, etc.)
- [ ] Audit trail node exists before END
- [ ] Data sources from SOP are referenced in retrieval nodes

**Nice to have:**
- [ ] Exception handling from SOP is addressed
- [ ] Record retention requirements are noted
- [ ] Customer contact guidelines are included (if applicable)

### Step 5: Fix Gaps

For each gap found, fix it in the Dify UI using browser automation:

**Adding input variables to START node:**
1. Click START node → settings panel opens
2. Click "+" button next to INPUT FIELD
3. Fill Variable Name and Label Name fields
4. Click Save

**Adding classifier classes:**
1. Click classifier node → settings panel opens
2. Scroll to class definitions
3. Click "+ Add Class"
4. Click directly on "Write your topic name" placeholder → type the class description
5. Scroll to NEXT STEP → click "SELECT NEXT STEP" for new class → select LLM node

**Adding LLM nodes for new paths:**
1. When adding a new class's next step, select "LLM" from dropdown
2. Triple-click node title → type new name
3. Click SYSTEM prompt area → type the prompt

**Editing existing node prompts:**
1. Click the node → settings panel opens
2. Click in the SYSTEM prompt area
3. Type or edit the prompt content

**Important browser automation rules:**
- Contenteditable divs: click precisely on the text area, then use `type`
- Canvas navigation: use `find` + `scroll_to`, NOT `scroll`
- Node renaming: `triple_click` title → `type` new name
- Don't use `cmd+a` — it selects the whole page, not just the field
- Dify auto-saves — no explicit save needed

### Step 6: Report

After fixing, provide a summary:
- Total nodes (before and after)
- Gaps found and fixed
- Gaps remaining (if any need manual intervention)
- Updated score (out of 10)
- Recommendation for the workflow

### Output Format

```
## SOP Workflow Analysis: {SOP_ID}

### Workflow: {workflow_name}
### URL: {workflow_url}

### Nodes Found: {count}
{node list with types}

### Gap Analysis
| # | Gap | SOP Reference | Severity | Status |
|---|-----|--------------|----------|--------|
| 1 | ... | Section X    | CRITICAL | FIXED  |

### Fixes Applied
{list of changes made}

### Score: {X}/10
### Remaining Manual Work
{anything that couldn't be automated}
```
