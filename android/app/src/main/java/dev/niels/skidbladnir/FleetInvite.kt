package dev.niels.skidbladnir

import java.nio.charset.StandardCharsets
import java.util.Base64
import java.util.concurrent.CompletableFuture
import java.util.concurrent.Executor
import kotlinx.serialization.Serializable
import kotlinx.serialization.SerializationException
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject

private const val FLEET_INVITE_KIND = "skidbladnir.fleet-invite.v1"
private const val MAXIMUM_FLEET_INVITE_BYTES = 4_096
private val FLEET_LABELS = listOf("Arch", "Devbox", "MacBook")

internal class PairingInviteToken private constructor(internal val encoded: String) {
    companion object {
        fun parse(candidate: String): PairingInviteToken? =
            candidate.takeIf(::isCanonicalBase64Url256)?.let(::PairingInviteToken)
    }

    override fun equals(other: Any?): Boolean = other is PairingInviteToken && encoded == other.encoded
    override fun hashCode(): Int = encoded.hashCode()
    override fun toString(): String = "PairingInviteToken(redacted)"
}

internal data class FleetInviteMachine(
    val machine: PairedMachine,
    val pairingInviteToken: PairingInviteToken,
)

internal data class FleetInvite(val machines: List<FleetInviteMachine>)

internal fun redeemFleetInvite(
    invite: FleetInvite,
    executor: Executor,
    redeem: (FleetInviteMachine) -> GatewayResult<PairingResponse>,
): CompletableFuture<List<MachineCredential>?> {
    val redeems = invite.machines.map { machine ->
        CompletableFuture.supplyAsync({ redeem(machine) }, executor)
    }
    return CompletableFuture.allOf(*redeems.toTypedArray()).thenApply {
        acceptPairingResults(invite, redeems.map { it.join() })
    }
}

internal fun acceptPairingResults(
    invite: FleetInvite,
    results: List<GatewayResult<PairingResponse>>,
): List<MachineCredential>? {
    if (results.size != invite.machines.size) return null
    val credentials = results.zip(invite.machines).map { (result, invited) ->
        val response = (result as? GatewayResult.Success)?.value ?: return null
        if (response.machine.handle != invited.machine.handle) return null
        val expectedPlatform = when (invited.machine.label.text) {
            "Arch", "Devbox" -> MachinePlatform.Linux
            "MacBook" -> MachinePlatform.Darwin
            else -> return null
        }
        if (response.machine.platform != expectedPlatform) return null
        MachineCredential(invited.machine, response.bearer)
    }
    return credentials.takeIf { fleet -> fleet.map { it.bearer }.distinct().size == fleet.size }
}

@Serializable
private data class WireFleetInvite(
    val kind: String,
    val machines: List<WireFleetInviteMachine>,
)

@Serializable
private data class WireFleetInviteMachine(
    val label: String,
    val origin: String,
    val machineHandle: String,
    val pairingInviteToken: String,
)

internal fun parseFleetInvite(encoded: String): FleetInvite? {
    if (encoded.toByteArray(StandardCharsets.UTF_8).size !in 1..MAXIMUM_FLEET_INVITE_BYTES) return null
    return try {
        val element = strictJsonObject(encoded)
        if (element.keys != setOf("kind", "machines") || element.values.any { it is JsonNull }) return null
        val machineElements = element.getValue("machines").jsonArray
        if (machineElements.size != FLEET_LABELS.size) return null
        if (machineElements.any { machine ->
                val fields = machine.jsonObject
                fields.keys != setOf("label", "origin", "machineHandle", "pairingInviteToken") ||
                    fields.values.any { it is JsonNull }
            }
        ) return null

        val wire = productJson.decodeFromJsonElement<WireFleetInvite>(element)
        if (wire.kind != FLEET_INVITE_KIND || wire.machines.map { it.label } != FLEET_LABELS) return null
        val machines = wire.machines.map { candidate ->
            val origin = MachineOrigin.parse(candidate.origin)
            if (origin == null || origin.encoded != candidate.origin) return null
            FleetInviteMachine(
                machine = PairedMachine(
                    handle = MachineHandle.parse(candidate.machineHandle) ?: return null,
                    label = MachineLabel.parse(candidate.label) ?: return null,
                    origin = origin,
                ),
                pairingInviteToken = PairingInviteToken.parse(candidate.pairingInviteToken) ?: return null,
            )
        }
        if (machines.map { it.machine.handle }.distinct().size != machines.size) return null
        if (machines.map { it.machine.origin }.distinct().size != machines.size) return null
        if (machines.map { it.pairingInviteToken }.distinct().size != machines.size) return null
        FleetInvite(machines)
    } catch (_: SerializationException) {
        // justify-ignore-error: scanner text is untrusted and every malformed JSON form has the
        // single frozen whole-fleet rejection outcome; the payload and decoder cause stay private.
        null
    } catch (_: IllegalArgumentException) {
        // justify-ignore-error: wrong JSON kinds and invalid owned values are the same rejected QR.
        null
    } catch (_: NoSuchElementException) {
        // justify-ignore-error: a missing required member is the same rejected QR.
        null
    }
}

internal fun isCanonicalBase64Url256(candidate: String): Boolean {
    if (candidate.length != 43 || candidate.any {
            it !in 'A'..'Z' && it !in 'a'..'z' && it !in '0'..'9' && it != '-' && it != '_'
        }
    ) return false
    val decoded = try {
        Base64.getUrlDecoder().decode(candidate)
    } catch (_: IllegalArgumentException) {
        // justify-ignore-error: invalid base64url is the expected rejection result of this parser.
        return false
    }
    return decoded.size == 32 && Base64.getUrlEncoder().withoutPadding().encodeToString(decoded) == candidate
}
