package dev.niels.skidbladnir

internal fun agentRuntimeFixture(
    provider: AgentProvider,
    pid: Long,
    profile: ProfileKey? = null,
    providerSession: ProviderSessionFacts? = null,
): AgentRuntime = AgentRuntime(
    provider, pid, "%4", "5678", AgentStatus(AgentState.Idle, AgentMethod.Native),
    AgentMethods(AgentMethod.Native, AgentMethod.Terminal, AgentMethod.Terminal), profile, providerSession,
)

internal fun agentFixtureJson(value: String): String = if (value.startsWith("{")) {
    """{"paneId":"%4","startIdentity":"5678","status":{"state":"idle","source":"native"},"methods":{"read":"native","send":"terminal","interrupt":"terminal"},""" + value.drop(1)
} else value
