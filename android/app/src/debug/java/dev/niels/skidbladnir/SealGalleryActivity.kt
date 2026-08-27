package dev.niels.skidbladnir

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

// The dwarf-seals acceptance instrument (docs/dwarf-seals.md): every
// catalogue seal at the 48dp acceptance size, for the hands-on pairwise
// distinguishability pass on the physical panel. Debug build only; reads the
// canonical catalogue mounted from the repo's catalog/ directory.

@Serializable
private data class GalleryEntry(val key: String, val displayName: String)

class SealGalleryActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val entries = Json { ignoreUnknownKeys = true }
            .decodeFromString<List<GalleryEntry>>(
                assets.open("characters.json").readBytes().decodeToString(),
            )
        setContent {
            MaterialTheme {
                Surface(modifier = Modifier.fillMaxSize(), color = Ink, contentColor = Bone) {
                    LazyVerticalGrid(
                        columns = GridCells.Adaptive(minSize = 72.dp),
                        modifier = Modifier.fillMaxSize().systemBarsPadding(),
                        verticalArrangement = Arrangement.spacedBy(10.dp),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        // The unstruck seal leads the catalogue so both
                        // acceptance comparisons are side-by-side rather than
                        // from memory: struck against unstruck, and lit against
                        // cold (forge-seal.md, "Acceptance and 80/20 gates").
                        items(listOf(true, false), key = { "unstruck-$it" }) { lit ->
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                ForgeSeal(canForge = lit, onClick = {})
                                Text(
                                    text = if (lit) "unstruck lit" else "unstruck cold",
                                    color = Muted,
                                    fontSize = 11.sp,
                                    fontFamily = NidavellirType.Data,
                                    modifier = Modifier.padding(top = 2.dp),
                                )
                            }
                        }
                        items(entries, key = { it.key }) { entry ->
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                DwarfPortrait(
                                    character = CharacterSummary(entry.key, entry.displayName),
                                    sealSize = 48.dp,
                                )
                                Text(
                                    text = entry.displayName,
                                    color = Muted,
                                    fontSize = 11.sp,
                                    fontFamily = NidavellirType.Data,
                                    modifier = Modifier.padding(top = 2.dp),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
