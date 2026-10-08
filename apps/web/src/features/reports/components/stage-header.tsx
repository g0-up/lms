import { versionLabel } from "@/shared/lib/format";
import { TableHead } from "@/shared/ui/table";
import type { ReportStage } from "../model/schemas";

/** Narrow stage column: the code on screen, the full "{stage} v{n}" as its title. */
export function StageHeader({ stage }: { stage: ReportStage }) {
  return (
    <TableHead className="text-right" title={`${stage.name} ${versionLabel({ no: stage.versionNo })}`}>
      {stage.code}
    </TableHead>
  );
}
