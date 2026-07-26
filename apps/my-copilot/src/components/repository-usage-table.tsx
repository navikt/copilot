"use client";

import { useState, useMemo, useCallback } from "react";
import { HStack, Pagination, Table, Search, Alert, VStack, BodyShort, Button, Tag } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableRow } from "@navikt/ds-react/Table";
import type { RepositoryUsage } from "@/lib/types";
import { formatNumber, formatMinutes } from "@/lib/format";

function CopyJsonButton({ data, label = "Kopier JSON" }: { data: unknown; label?: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(JSON.stringify(data, null, 2)).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }, [data]);
  return (
    <Button variant="tertiary-neutral" size="xsmall" onClick={handleCopy}>
      {copied ? "✓ Kopiert" : label}
    </Button>
  );
}

const PAGE_SIZE = 15;

type SortKey =
  | "repo_name"
  | "repo_visibility"
  | "pr_created_by_copilot"
  | "pr_reviewed_by_copilot"
  | "pr_total_merged"
  | "pr_avg_median_minutes_to_merge";

interface RepositoryUsageTableProps {
  repositories: RepositoryUsage[];
}

// nb-NO label for the GitHub visibility enum (PUBLIC | INTERNAL).
// Private repos are excluded server-side, so PRIVATE should never reach here.
function visibilityLabel(visibility: string): string {
  switch (visibility.toUpperCase()) {
    case "PUBLIC":
      return "Offentlig";
    case "INTERNAL":
      return "Intern";
    case "PRIVATE":
      return "Privat";
    default:
      return visibility;
  }
}

export default function RepositoryUsageTable({ repositories }: RepositoryUsageTableProps) {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  // Default sort mirrors the API's ordering: most Copilot-authored PRs first.
  const [sortKey, setSortKey] = useState<SortKey>("pr_created_by_copilot");
  const [sortDirection, setSortDirection] = useState<"ascending" | "descending">("descending");

  const filteredRepos = useMemo(() => {
    if (!search.trim()) return repositories;
    const query = search.toLowerCase();
    return repositories.filter((r) => `${r.repo_owner}/${r.repo_name}`.toLowerCase().includes(query));
  }, [repositories, search]);

  const sortedRepos = useMemo(() => {
    return [...filteredRepos].sort((a, b) => {
      if (sortKey === "repo_name") {
        const aName = `${a.repo_owner}/${a.repo_name}`;
        const bName = `${b.repo_owner}/${b.repo_name}`;
        return sortDirection === "ascending" ? aName.localeCompare(bName) : bName.localeCompare(aName);
      }
      if (sortKey === "repo_visibility") {
        return sortDirection === "ascending"
          ? a.repo_visibility.localeCompare(b.repo_visibility)
          : b.repo_visibility.localeCompare(a.repo_visibility);
      }
      // Numeric keys, including the nullable median — nulls sort to the bottom
      // regardless of direction so "no data" rows never top the list.
      const aVal = a[sortKey];
      const bVal = b[sortKey];
      if (aVal == null && bVal == null) return 0;
      if (aVal == null) return 1;
      if (bVal == null) return -1;
      const diff = aVal - bVal;
      return sortDirection === "ascending" ? diff : -diff;
    });
  }, [filteredRepos, sortKey, sortDirection]);

  const totalPages = Math.ceil(sortedRepos.length / PAGE_SIZE);
  const pageRepos = sortedRepos.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  function handleSort(key: string) {
    if (sortKey === key) {
      setSortDirection((d) => (d === "ascending" ? "descending" : "ascending"));
    } else {
      setSortKey(key as SortKey);
      setSortDirection("descending");
    }
    setPage(1);
  }

  function handleSearch(value: string) {
    setSearch(value);
    setPage(1);
  }

  return (
    <VStack gap="space-16">
      <Alert variant="info" size="small">
        Tallene er summert over hele perioden med data. Private repositorier er utelatt, og repositorier med færre enn 5
        pull requests totalt vises ikke (personvern).
      </Alert>

      <HStack gap="space-8" align="end" wrap>
        <Search label="Søk" size="small" variant="simple" value={search} onChange={handleSearch} className="max-w-xs" />
        <CopyJsonButton data={sortedRepos} label="📋 JSON" />
      </HStack>

      <div className="overflow-x-auto">
        <Table size="small" sort={{ orderBy: sortKey, direction: sortDirection }} onSortChange={handleSort}>
          <TableHeader>
            <TableRow>
              <Table.ColumnHeader scope="col" sortKey="repo_name" sortable>
                Repositorium
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="repo_visibility" sortable>
                Synlighet
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="pr_created_by_copilot" sortable align="right">
                Copilot-forfattet
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="pr_reviewed_by_copilot" sortable align="right">
                Copilot-reviewet
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="pr_total_merged" sortable align="right">
                Merget totalt
              </Table.ColumnHeader>
              <Table.ColumnHeader scope="col" sortKey="pr_avg_median_minutes_to_merge" sortable align="right">
                Median tid til merge
              </Table.ColumnHeader>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pageRepos.map((repo) => (
              <TableRow key={repo.repo_id}>
                <TableDataCell>
                  <a
                    href={`https://github.com/${repo.repo_owner}/${repo.repo_name}`}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="text-blue-600 hover:underline"
                  >
                    {repo.repo_owner}/{repo.repo_name}
                  </a>
                </TableDataCell>
                <TableDataCell>
                  <Tag variant={repo.repo_visibility.toUpperCase() === "PUBLIC" ? "success" : "neutral"} size="xsmall">
                    {visibilityLabel(repo.repo_visibility)}
                  </Tag>
                </TableDataCell>
                <TableDataCell align="right">{formatNumber(repo.pr_created_by_copilot)}</TableDataCell>
                <TableDataCell align="right">{formatNumber(repo.pr_reviewed_by_copilot)}</TableDataCell>
                <TableDataCell align="right">{formatNumber(repo.pr_total_merged)}</TableDataCell>
                <TableDataCell align="right">{formatMinutes(repo.pr_avg_median_minutes_to_merge)}</TableDataCell>
              </TableRow>
            ))}
            {pageRepos.length === 0 && (
              <TableRow>
                <TableDataCell colSpan={6}>
                  <BodyShort className="text-gray-500 text-center">
                    {search ? "Ingen repositorier funnet for søket ditt." : "Ingen repositoriedata tilgjengelig ennå."}
                  </BodyShort>
                </TableDataCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      {totalPages > 1 && (
        <HStack justify="center">
          <Pagination page={page} onPageChange={setPage} count={totalPages} size="small" />
        </HStack>
      )}
    </VStack>
  );
}
