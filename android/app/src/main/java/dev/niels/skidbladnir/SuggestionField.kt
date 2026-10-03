package dev.niels.skidbladnir

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.isImeVisible
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.minimumInteractiveComponentSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/** Text stays with the form. The domain supplies rows and the only value IME may accept. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
internal fun SuggestionField(
    text: String,
    label: String,
    textStyle: TextStyle,
    enabled: Boolean,
    expanded: Boolean,
    onExpandedChange: (Boolean) -> Unit,
    onChange: (String) -> Unit,
    literal: String?,
    onAccept: (String) -> Boolean,
    onNext: () -> Unit,
    onFocus: () -> Unit,
    maxSuggestionsHeight: Dp,
    message: String?,
    invalid: Boolean,
    modifier: Modifier,
    choices: LazyListScope.((String) -> Unit) -> Unit,
) {
    val suggestions = rememberLazyListState()
    val bringIntoView = remember { BringIntoViewRequester() }
    val focus = LocalFocusManager.current
    val keyboard = LocalSoftwareKeyboardController.current
    val imeVisible = WindowInsets.isImeVisible
    LaunchedEffect(text, expanded) {
        if (expanded) suggestions.scrollToItem(0)
    }
    LaunchedEffect(expanded, imeVisible, message, maxSuggestionsHeight) {
        if (expanded) {
            withFrameNanos { }
            bringIntoView.bringIntoView()
        }
    }
    if (expanded && enabled) {
        ModalBackHandler {
            focus.clearFocus()
            keyboard?.hide()
            if (!imeVisible) onExpandedChange(false)
        }
    }
    fun accept(value: String): Boolean {
        if (!onAccept(value)) return false
        onExpandedChange(false)
        focus.clearFocus()
        keyboard?.hide()
        return true
    }
    Column(
        Modifier.fillMaxWidth().bringIntoViewRequester(bringIntoView),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        OutlinedTextField(
            value = text,
            onValueChange = {
                onChange(it)
                onExpandedChange(true)
            },
            enabled = enabled,
            singleLine = true,
            label = { Text(label, maxLines = 1, overflow = TextOverflow.Ellipsis) },
            textStyle = textStyle,
            isError = invalid,
            keyboardOptions = KeyboardOptions(
                capitalization = KeyboardCapitalization.None,
                autoCorrectEnabled = false,
                imeAction = ImeAction.Next,
            ),
            keyboardActions = KeyboardActions(onNext = {
                literal?.let { if (accept(it)) onNext() }
            }),
            modifier = modifier.fillMaxWidth().onFocusChanged {
                if (it.isFocused) {
                    onExpandedChange(true)
                    onFocus()
                }
            },
        )
        message?.let {
            Text(it, color = if (invalid) Ember else Muted,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
        }
        if (expanded && enabled) {
            LazyColumn(
                Modifier.fillMaxWidth().height(maxSuggestionsHeight.coerceAtMost(240.dp)),
                state = suggestions,
            ) {
                choices { accept(it) }
            }
        }
    }
}

@Composable
internal fun SuggestionChoice(
    description: String,
    onClick: () -> Unit,
    content: @Composable () -> Unit,
) {
    TextButton(
        onClick = onClick,
        shape = NidavellirShapes.Chip,
        modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)
            .minimumInteractiveComponentSize().semantics { contentDescription = description },
    ) {
        content()
    }
}

@Composable
internal fun SuggestionTextChoice(label: String, description: String, onClick: () -> Unit) {
    SuggestionChoice(description, onClick) {
        Text(label, color = Bone, fontFamily = NidavellirType.Data, maxLines = 1,
            overflow = TextOverflow.Ellipsis)
    }
}
