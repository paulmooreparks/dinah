# Managed root identity

The Windows workflow at 6cc7c49 refused legitimate managed-workspace reads and writes when its temporary directory used an 8.3 short name. The managed exception compared the supplied root with EvalSymlinks output to infer that no link existed. Expansion of a short name changes that spelling without redirecting the object.

The exception will inspect every existing path component with Lstat and refuse symbolic links and Windows reparse points. It then uses os.SameFile against the held identity chain, as the protected-path checks already do. A missing or unreadable component fails closed. Path spelling after resolution is never evidence of a link. This preserves the refusal of redirected managed folders while allowing short names and case aliases for ordinary folders.

The Windows regression creates a long-named temporary directory, obtains its short path through the documented GetShortPathNameW API, and exercises both a read and a write with that state root. The existing junction and managed-root guards remain required. Hosts without short-name support cannot establish this scenario and report it explicitly.

The OS contracts are documented at https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getshortpathnamew and https://learn.microsoft.com/en-us/windows/win32/fileio/reparse-points-and-file-operations. Go documents Lstat, SameFile and FileInfo.Sys at https://pkg.go.dev/os. The Windows Sys value supplies Win32FileAttributeData, whose FILE_ATTRIBUTE_REPARSE_POINT bit is checked explicitly.
