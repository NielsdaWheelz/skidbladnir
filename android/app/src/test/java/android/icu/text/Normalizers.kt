package android.icu.text

// The JVM suite binds the public platform namespace to the same upstream ICU engine.
// Only namespace forwarding lives here; normalization and age properties stay in ICU.
class Normalizer2 private constructor(private val delegate: com.ibm.icu.text.Normalizer2) {
    fun normalize(source: CharSequence): String = delegate.normalize(source)

    companion object {
        @JvmStatic
        fun getNFCInstance(): Normalizer2 = Normalizer2(com.ibm.icu.text.Normalizer2.getNFCInstance())
    }
}

class UnicodeSet(pattern: String) {
    private val delegate = com.ibm.icu.text.UnicodeSet(pattern)
    enum class SpanCondition { CONTAINED, NOT_CONTAINED, SIMPLE }

    fun span(source: CharSequence, start: Int, condition: SpanCondition): Int =
        delegate.span(source, start, com.ibm.icu.text.UnicodeSet.SpanCondition.valueOf(condition.name))

    fun freeze(): UnicodeSet {
        delegate.freeze()
        return this
    }
}
