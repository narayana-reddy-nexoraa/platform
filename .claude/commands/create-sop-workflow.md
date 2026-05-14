# Create SOP Workflow in Dify Studio

Create a new SOP workflow in the Nexoraa AI Studio using "Create from SOP" feature.

## Arguments
- $ARGUMENTS: The SOP ID to create (e.g., "FS-01", "INS-01", "CPR-01"). If "all", create all missing workflows. If empty, ask the user which SOP to create.

## Instructions

### Step 1: Check What Exists

1. Read the SOP registry from `internal/sop/registry/registry.go` to get the full list of 25 SOPs
2. Navigate to `http://34.228.19.10:3000/apps?isCreatedByMe=true` using Chrome browser tools
3. Read the page to list all existing workflow cards
4. Compare — identify which SOPs are missing workflows

If a specific SOP ID was provided, check if it already exists. If it does, ask the user if they want to recreate or skip.

### Step 2: Prepare SOP Content

For the SOP to create:
1. Read the individual SOP file from `/Users/narayana-nexoraa/Developer/HSD/SOP/individual/{SOP_ID}.md`
2. Read the SOP definition from `internal/sop/registry/` to get structured data (steps, compliance, inputs)
3. Prepare a comprehensive prompt combining both sources

### Step 3: Create Workflow via Prompt

Since Document Upload requires manual file picker interaction, use the Prompt tab:

1. Click "Create from SOP" button on the apps page
2. The "Create from SOP" dialog opens with Prompt and Document Upload tabs
3. Click the Prompt tab (should be default)
4. Click in the message input field at the bottom
5. Type a detailed prompt describing the SOP workflow

**Prompt template:**
```
Create a workflow for SOP {SOP_ID}: {SOP_NAME} for {INDUSTRY}.

{SOP_DESCRIPTION}

Required workflow steps:
1) {INTAKE_NAME}: {INTAKE_DESC}
   Input variables: {list all input fields from SOP Section 6.1}
2) {DATA_RETRIEVAL_NAME}: {DATA_RETRIEVAL_DESC}
   Data sources: {list from SOP}
3) {CLASSIFICATION_NAME}: {CLASSIFICATION_DESC}
   Risk tiers: {list all classification levels from SOP Section 7}
4) {DECISIONING_NAME}: {DECISIONING_DESC}
   HITL gate required: {yes/no based on SOP controls}
   Classes: {list all decision classes with their actions from SOP Section 8}
5) {EXECUTION_NAME}: {EXECUTION_DESC}
   Target systems: {list from SOP}
6) Audit Trail: Log all actions immutably with compliance evidence
   Retention: {years from SOP Section 13}

Compliance: {list frameworks from SOP}
Controls: {list from SOP Section 12}

The classifier node MUST have {N} classes matching the SOP risk tiers, not just 2.
Include a Variable Aggregator after the classifier branches to merge all paths.
Include an Impact/Disposition node after the aggregator.
End with an END node that outputs the final result.
```

6. Click "Send" button
7. Wait for the AI to generate the workflow (10-15 seconds)
8. Review the generated diagram
9. Click "Import Workflow"

### Step 4: Verify and Fix

After import, run the analysis process:
1. Read all nodes in the generated workflow
2. Compare against SOP definition
3. Fix any gaps (missing input variables, insufficient classifier classes, missing HITL gates)

Use the same fixing patterns from the analyse-sop-workflow command.

### Step 5: Report

```
## Created SOP Workflow: {SOP_ID}

### Workflow: {workflow_name}
### URL: {workflow_url}
### Nodes: {count}

### Auto-generated nodes:
{list}

### Fixes applied after generation:
{list}

### Score: {X}/10
```

### If $ARGUMENTS is "all"

Loop through all 25 SOPs and create missing ones:
1. Get list of existing workflows from Dify
2. Get list of all 25 SOP IDs from registry
3. For each missing SOP: create → verify → fix → report
4. At the end, provide a summary table of all 25 workflows and their status

**Important:** Between each workflow creation, wait 5 seconds to avoid rate limiting the Dify API.

### Browser Automation Rules (from memory)

- Contenteditable divs: click precisely on text area → `type`
- Canvas navigation: use `find` + `scroll_to`, NOT `scroll`
- Node renaming: `triple_click` title → `type`
- Don't use `cmd+a` — selects whole page
- Dify auto-saves — no explicit save needed
- For classifier class descriptions: click on placeholder text → `type`
