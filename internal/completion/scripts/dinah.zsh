#compdef dinah dinah.exe
# dinah completion for zsh, protocol __DINAH_PROTOCOL__.
# Load it from ~/.zshrc, after compinit:  eval "$(dinah completion zsh)"
#
# The script runs the program named by the first word of the line, so the
# candidates always come from the binary that will run the command, and it
# adds them with compadd -U, so zsh inserts what the callback matched rather
# than filtering the list a second time.
_dinah() {
  emulate -L zsh
  local prog=${(Q)words[1]} out header mode line insert desc
  local -a lines inserts shown
  [[ $prog == '~/'* ]] && prog=$HOME/${prog#'~/'}
  out=$("$prog" __complete __DINAH_PROTOCOL__ zsh -- "${(@Q)words[2,CURRENT-1]}" "${(Q)PREFIX}" 2>/dev/null) || return 1
  lines=("${(@f)out}")
  header=${lines[1]}
  [[ $header == ('dinah-complete __DINAH_PROTOCOL__ '(words|nospace|files|dirs)) ]] || return 1
  mode=${header##* }
  case $mode in
    files) _files; return ;;
    dirs) _files -/; return ;;
  esac
  for line in "${(@)lines[2,-1]}"; do
    insert=${line%%$'\t'*}
    [[ -n $insert ]] || continue
    desc=${line#*$'\t'}
    inserts+=("$insert")
    if [[ -n $desc ]]; then shown+=("$insert -- $desc"); else shown+=("$insert"); fi
  done
  (( ${#inserts} )) || return 1
  if [[ $mode == nospace ]]; then
    compadd -U -S '' -l -d shown -- "${inserts[@]}"
  else
    compadd -U -l -d shown -- "${inserts[@]}"
  fi
}
(( $+functions[compdef] )) && compdef _dinah dinah dinah.exe
