import {
  BlockTypeSelect,
  BoldItalicUnderlineToggles,
  CodeToggle,
  CreateLink,
  DiffSourceToggleWrapper,
  InsertCodeBlock,
  InsertImage,
  InsertTable,
  InsertThematicBreak,
  ListsToggle,
  Separator,
  StrikeThroughSupSubToggles,
  UndoRedo,
} from "@mdxeditor/editor";

/** Only the commands the server renders: no underline, sub/superscript, task lists or raw HTML. */
export function EditorToolbar() {
  return (
    <DiffSourceToggleWrapper options={["rich-text", "source"]}>
      <UndoRedo />
      <Separator />
      <BlockTypeSelect />
      <Separator />
      <BoldItalicUnderlineToggles options={["Bold", "Italic"]} />
      <StrikeThroughSupSubToggles options={["Strikethrough"]} />
      <CodeToggle />
      <Separator />
      <ListsToggle options={["bullet", "number"]} />
      <Separator />
      <CreateLink />
      <InsertImage />
      <InsertTable />
      <InsertThematicBreak />
      <InsertCodeBlock />
    </DiffSourceToggleWrapper>
  );
}
