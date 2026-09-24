# dinah completion for bash, protocol __DINAH_PROTOCOL__.
# Load it from ~/.bashrc:  eval "$(dinah completion bash)"
#
# The script runs the program named by the first word of the line, so the
# candidates always come from the binary that will run the command. It sends
# the line only when every character on it is ASCII and the cursor is at its
# end, because only then does COMP_POINT equal ${#COMP_LINE} whichever unit
# bash counts in. In every other case it offers nothing and runs nothing.
_dinah_complete() {
    local prog=${COMP_WORDS[0]} out header mode body line
    COMPREPLY=()
    [[ $COMP_LINE == *[![:ascii:]]* ]] && return 0
    (( COMP_POINT == ${#COMP_LINE} )) || return 0
    [[ $prog == "~/"* ]] && prog=$HOME/${prog#"~/"}
    out=$(MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' "$prog" __complete __DINAH_PROTOCOL__ bash "$COMP_WORDBREAKS" "$COMP_LINE" 2>/dev/null) || return 0
    header=${out%%$'\n'*}
    case $header in
        'dinah-complete __DINAH_PROTOCOL__ words' | 'dinah-complete __DINAH_PROTOCOL__ nospace' | \
        'dinah-complete __DINAH_PROTOCOL__ files' | 'dinah-complete __DINAH_PROTOCOL__ dirs') ;;
        *) return 0 ;;
    esac
    mode=${header##* }
    if type compopt >/dev/null 2>&1; then
        case $mode in
            nospace) compopt -o nospace ;;
            files) compopt -o default; return 0 ;;
            dirs) compopt -o dirnames; return 0 ;;
        esac
    fi
    [[ $out == *$'\n'* ]] || return 0
    body=${out#*$'\n'}
    while IFS= read -r line; do
        line=${line%%$'\t'*}
        [[ -n $line ]] && COMPREPLY+=("$line")
    done <<< "$body"
    return 0
}
complete -F _dinah_complete dinah dinah.exe
