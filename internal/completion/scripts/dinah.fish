# dinah completion for fish, protocol __DINAH_PROTOCOL__.
# Load it from ~/.config/fish/config.fish:  dinah completion fish | source
#
# The script runs the program named by the first word of the line, so the
# candidates always come from the binary that will run the command. fish reads
# a candidate and its description from one line separated by a TAB, which is
# the form the callback writes.
function __dinah_complete
    set -l tokens (commandline -opc)
    set -l current (commandline -ct)
    set -l prog $tokens[1]
    set -e tokens[1]
    if string match -q -- '~/*' $prog
        set prog $HOME/(string sub -s 3 -- $prog)
    end
    set -l out (env MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' $prog __complete __DINAH_PROTOCOL__ fish -- $tokens $current 2>/dev/null)
    or return
    string match -qr -- '^dinah-complete __DINAH_PROTOCOL__ (words|nospace|files|dirs)$' $out[1]
    or return
    set -l mode (string split -r -m1 ' ' -- $out[1])[2]
    set -e out[1]
    switch $mode
        case files
            for p in $current*
                printf '%s\n' $p
            end
        case dirs
            for p in $current*
                test -d $p; and printf '%s/\n' $p
            end
        case '*'
            for l in $out
                test -n "$l"; and printf '%s\n' $l
            end
    end
end
complete -c dinah -f -a '(__dinah_complete)'
complete -c dinah.exe -f -a '(__dinah_complete)'
