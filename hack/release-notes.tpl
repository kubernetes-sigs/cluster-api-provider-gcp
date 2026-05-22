{{- $CurrentRevision := .CurrentRevision -}}
{{- $PreviousRevision := .PreviousRevision -}}
## Changelog since {{$PreviousRevision}}

{{with .NotesWithActionRequired -}}
## Urgent Upgrade Notes

### (No, really, you MUST read this before you upgrade)

{{range .}}{{println "-" .}} {{end}}
{{end}}

{{- if .Notes -}}
## Changes by Kind
{{ range .Notes}}
### {{.Kind | prettyKind}}

{{range $note := .NoteEntries }}{{println "-" $note}}{{end}}
{{- end -}}
{{- end -}}
