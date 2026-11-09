# website previews blocked by vercel account

problem: vercel refuses the repository's website-preview deployment because its
account is blocked. the account's underlying cause has not been inspected.

impact: website previews cannot qualify. installed skid services are independent.

evidence: the vercel check on documentation-only pr 201 reports `Account is blocked.`
and links to [vercel's account-block guidance](https://vercel.com/knowledge/why-is-my-account-deployment-blocked).
source verification and the credential scan pass.

resolved when: the owner resolves the account restriction and a new preview
deployment succeeds. no website, billing or account change is authorized here.
