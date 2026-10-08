export { routes } from "./routes";

// Shared with the courses feature, which has the same version lifecycle and list layout.
export { stagesQuery } from "./hooks/use-stages";
export { relatedKeys as contentKeys } from "./api/stages-api";
export type { StageList, StageListItem, StageVersionSummary } from "./model/schemas";
export { stageListSchema, versionStatusSchema } from "./model/schemas";
export { VERSION_IMMUTABLE_MESSAGE } from "./model/stage-rules";
export { draftOf, latestPublished, selectVersion, sortVersions, toVersionRef, type VersionLike } from "./model/select-version";
export { moveItem, orderByIds } from "./model/order";
export { codeNameSchema, type CodeNameOutput, type CodeNameValues } from "./model/code-name-form";
export { CodeNameDialog } from "./components/code-name-dialog";
export { GuardedButton } from "./components/guarded-button";
export { ItemBody, ItemMeta, ItemTitle, ItemTitleLink, List, ListItem, Ord } from "./components/list";
export { LockNote } from "./components/lock-note";
export { OrderButtons } from "./components/order-buttons";
export { navigateFromRow } from "./components/row-link";
export { VersionActions, type VersionActionsProps } from "./components/stage-actions-bar";
export { VersionCard } from "./components/version-card";
