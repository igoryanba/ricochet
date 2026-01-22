/**
 * Ricochet Maintenance Script: Auto-Close Stale Issues
 *
 * Usage:
 *   ricochet-agent run auto-close-stale.ts
 *
 * Description:
 *   Finds issues with no activity for 30 days and closes them.
 */

interface Issue {
  number: number;
  updated_at: string;
  title: string;
}

async function main() {
  console.log("🔍 Scanning for stale issues...");
  
  // 1. Fetch Issues via MCP (Pseudo-code as this runs inside Agent environment usually, 
  // but written as standalone TS for portability in this template)
  // In a real usage, the Agent would execute this or interpret it.
  
  // Assuming 'gh' CLI is available for this script
  const { $ } = await import("bun");
  
  const staleDays = 30;
  const dateThreshold = new Date();
  dateThreshold.setDate(dateThreshold.getDate() - staleDays);
  
  console.log(`Checking for issues older than ${dateThreshold.toISOString()}`);
  
  // Fetch open issues sorted by update time
  const issuesJson = await $`gh issue list --state open --json number,updatedAt,title --limit 100`.text();
  const issues: any[] = JSON.parse(issuesJson);
  
  for (const issue of issues) {
    const updated = new Date(issue.updatedAt);
    if (updated < dateThreshold) {
      console.log(`Closing stale issue #${issue.number}: ${issue.title}`);
      await $`gh issue close ${issue.number} --comment "Closing due to inactivity."`;
    }
  }
}

main();
