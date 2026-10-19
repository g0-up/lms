import { Field } from "@/shared/ui/field";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { LESSON_UNIT_HELP_CREATE, LESSON_UNIT_HELP_EDIT } from "../model/lesson-form";
import type { Unit } from "../model/schemas";

export interface UnitSelectProps {
  units: readonly Pick<Unit, "id" | "title">[];
  value: string;
  onChange: (unitId: string) => void;
  onBlur: () => void;
  /** Editing an existing lesson: moving it sends it to the end of the other unit. */
  editing: boolean;
  disabled?: boolean;
  error?: string;
}

/** "Bài học" select of the lesson form: the version's units as "{i}. {title}". */
export function UnitSelect({ units, value, onChange, onBlur, editing, disabled, error }: UnitSelectProps) {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <Field label="Bài học" required help={editing ? LESSON_UNIT_HELP_EDIT : LESSON_UNIT_HELP_CREATE} error={error}>
        <SelectTrigger onBlur={onBlur}>
          <SelectValue placeholder="Chọn bài học" />
        </SelectTrigger>
      </Field>
      <SelectContent>
        {units.map((unit, i) => (
          <SelectItem key={unit.id} value={unit.id}>
            {`${String(i + 1)}. ${unit.title}`}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
