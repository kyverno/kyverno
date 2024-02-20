{{ define "members" }}

  {{/* . is a apiType */}}
  {{ range .GetMembers }}
    {{/* . is a apiMember */}}
    {{ if not .Hidden }}
      <tr>
        <td><code>{{ .FieldName }}</code>
          {{ if not .IsOptional }}
          <span style="color:blue;"> *</span>
          {{ end }}
          </br>

          {{/* Link for type reference */}}
          {{ with .GetType }}
            {{ if .Link }}
              <a href="{{ .Link }}">
                <span style="font-family: monospace">{{ .DisplayName }}</span>
              </a>
            {{ else }}
              <span style="font-family: monospace">{{ .DisplayName }}</span>
            {{ end }}
          {{ end }}
        </td>
        <td>
          {{ if .IsInline }}
            <p>(Members of <code>{{ .FieldName }}</code> are embedded into this type.)</p>
          {{ end}}

          {{ .GetComment }}

          {{ if and (eq (.GetType.Name.Name) "ObjectMeta") }}
            Refer to the Kubernetes API documentation for the fields of the
            <code>metadata</code> field.
          {{ end }}

          {{ if or (eq .FieldName "spec") }}
            <br/>
            <br/>
            <table>
              {{ template "members" .GetType }}
            </table>
          {{ end }}
        </td>
      </tr>
    {{ end }}
  {{ end }}
{{ end }}
