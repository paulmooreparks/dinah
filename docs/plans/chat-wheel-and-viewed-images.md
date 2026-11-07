# Chat wheel intent and viewed images

Paul reported upward wheel movement snapping back to the transcript end on 5 November 2026. A browser regression using an eight-pixel upward wheel movement reproduces the loss of position. The transcript releases following on input, but its next visibility update re-enables following inside the 24-pixel end tolerance. The correction retains upward intent until the reader leaves that tolerance or reverses wheel direction. Explicit navigation to the latest message resumes following. Position preservation must also respect the transcript following state inside that tolerance.

## Viewed images

Paul asked whether provider image-view operations can expose the viewed image in Dinah. The locally generated Codex app-server schema defines ImageViewThreadItem with required id, path and type fields. Dinah retains the raw item but gives this item only a generic tool title. Its tool renderer extracts text and ignores image content blocks. The existing chat gallery already opens images through the media viewer.

The proposed implementation normalizes documented provider image results into a common viewed-image representation. Tool rows show a thumbnail and filename with an action opening the existing media viewer. Codex imageView supplies a local path; other adapters must use documented image blocks or explicit image references rather than guesses from command text. The host must validate and resolve local files through its resource access rules. A path reference displays the current file, which may differ from the bytes originally viewed; retained provider image bytes can represent the original image. Missing files need an explicit unavailable state. Replay and repeated tool updates must avoid duplicate thumbnails. This image feature has been investigated but is not implemented in this change.

## Verification

The original small-wheel regression failed before the change. The final chat-tail, chat-scroll-place and chat-unread suites passed. The focused regression covers an eight-pixel upward movement followed by content growth, continued scrollback, and explicit return to the latest content. An earlier absolute-scrollTop assertion during provider text re-rendering observed a 45-pixel shift while following remained false. That broader layout shift remains uncharacterized; the final regression isolates content growth from text re-rendering. Evidence remains under build/wheel-before, build/wheel-final, and build/wheel-verified. These source changes have not been built or installed.
