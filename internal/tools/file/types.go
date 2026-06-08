package file

// ListDirInput is the schema for listing a workspace directory.
// ListDirInput 是列出工作区目录的工具入参。
type ListDirInput struct {
	Path string `json:"path" jsonschema:"required" jsonschema_description:"Workspace-relative directory path to list."`
}

// DirEntry is a compact directory entry.
// DirEntry 是紧凑的目录项。
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ListDirOutput contains directory entries.
// ListDirOutput 包含目录项。
type ListDirOutput struct {
	Path    string     `json:"path"`
	Entries []DirEntry `json:"entries"`
}

// ReadFileInput is the schema for reading a workspace file.
// ReadFileInput 是读取工作区文件的工具入参。
type ReadFileInput struct {
	Path     string `json:"path" jsonschema:"required" jsonschema_description:"Workspace-relative file path to read."`
	MaxBytes int64  `json:"max_bytes,omitempty" jsonschema_description:"Optional maximum bytes to return."`
}

// ReadFileOutput contains file content and truncation metadata.
// ReadFileOutput 包含文件内容和截断信息。
type ReadFileOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// WriteFileInput is shared by create and replace file tools.
// WriteFileInput 是创建与替换文件工具的共享入参。
type WriteFileInput struct {
	Path    string `json:"path" jsonschema:"required" jsonschema_description:"Workspace-relative file path."`
	Content string `json:"content" jsonschema:"required" jsonschema_description:"Complete UTF-8 file content."`
	DryRun  bool   `json:"dry_run,omitempty" jsonschema_description:"When true, describe the change without writing."`
}

// PatchFileInput applies a simple full-content guarded patch.
// PatchFileInput 应用带原内容保护的简单补丁。
type PatchFileInput struct {
	Path    string `json:"path" jsonschema:"required"`
	OldText string `json:"old_text" jsonschema:"required" jsonschema_description:"Text expected to exist exactly once."`
	NewText string `json:"new_text" jsonschema:"required" jsonschema_description:"Replacement text."`
	DryRun  bool   `json:"dry_run,omitempty"`
}

// DeleteFileInput deletes one workspace file.
// DeleteFileInput 删除一个工作区文件。
type DeleteFileInput struct {
	Path   string `json:"path" jsonschema:"required"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// MutationOutput describes a file mutation result.
// MutationOutput 描述文件变更结果。
type MutationOutput struct {
	Path    string `json:"path"`
	Changed bool   `json:"changed"`
	Message string `json:"message"`
}
