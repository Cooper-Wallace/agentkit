---
name: llm-structured-output
description: Guaranteed JSON schema extraction and structured Pydantic models with local LLMs (Ollama) and cloud APIs.
---

# LLM Structured Output & Schema Enforcement Standards

This skill provides production patterns for extracting guaranteed, syntactically and semantically validated structured data (JSON/Pydantic) from local LLMs (Ollama, vLLM) and frontier cloud models (OpenAI, Anthropic, Gemini).

---

## 1. Pydantic Model Architecture

All structured extraction targets must be defined as Pydantic v2 models with rich constraints, field descriptions, and custom validators.

```python
from typing import Optional, Literal
from pydantic import BaseModel, Field, field_validator, model_validator
from datetime import date

class FinancialMetric(BaseModel):
    metric_name: str = Field(description="Standardized name of the financial metric (e.g., 'revenue', 'ebitda')")
    value: float = Field(description="Numerical value reported, adjusted for units")
    unit: Literal["USD", "EUR", "shares", "percentage", "ratio"] = Field(description="Unit of measurement")
    fiscal_period: str = Field(description="Quarter or annual tag, e.g., 'Q3-2024' or 'FY-2023'")
    confidence_score: float = Field(ge=0.0, le=1.0, description="Confidence in extraction accuracy (0.0 to 1.0)")

class SECFilingExtraction(BaseModel):
    company_name: str = Field(description="Canonical corporate entity name")
    ticker: Optional[str] = Field(None, pattern=r"^[A-Z]{1,5}$", description="Stock ticker symbol")
    filing_date: date = Field(description="Date the report was filed with the SEC")
    report_type: Literal["10-K", "10-Q", "8-K"] = Field(description="SEC form classification")
    metrics: list[FinancialMetric] = Field(default_factory=list, description="Extracted quantitative metrics")
    key_risks: list[str] = Field(default_factory=list, description="Primary business risks explicitly cited")

    @field_validator("metrics")
    @classmethod
    def validate_non_empty_metrics(cls, v):
        if not v:
            raise ValueError("Filing extraction must contain at least one valid financial metric")
        return v
```

---

## 2. Multi-Provider Enforcement Strategies

### Frontier Cloud APIs (OpenAI / Anthropic / Gemini)
- **OpenAI Structured Outputs**: Use `response_format={"type": "json_schema", "json_schema": {"strict": True, "schema": ...}}`. Guarantees 100% adherence to the generated schema via constrained decoding.
- **Anthropic Tools**: Define schema as an input tool and enforce tool use via `tool_choice={"type": "tool", "name": "extract_filing"}`.
- **Instructor Framework**: Provides a unified, ergonomic abstraction across providers with automatic retry loops:

```python
import instructor
from openai import OpenAI

# Works identically with OpenAI, Ollama (via OpenAI-compatible endpoint), or Anthropic
client = instructor.from_openai(OpenAI())

def extract_structured_filing(document_text: str) -> SECFilingExtraction:
    return client.chat.completions.create(
        model="gpt-4o-mini",
        response_model=SECFilingExtraction,
        max_retries=3,
        messages=[
            {"role": "system", "content": "You are a quantitative extraction specialist parsing SEC filings."},
            {"role": "user", "content": f"Extract all metrics from the following text:\n\n{document_text}"}
        ]
    )
```

### Local LLMs (Ollama / vLLM / Outlines)
When running on local workstations or air-gapped systems:
1. **Ollama `format="json"`**: Forces JSON syntax token generation.
2. **Grammar-Constrained Decoding (Outlines / GBNF)**: Compiles the Pydantic JSON Schema into a finite state machine (FSM) or grammar that masks invalid next tokens during sampling:
   ```python
   # Example with Outlines on local HuggingFace/vLLM models
   # import outlines
   # generator = outlines.generate.json(model, SECFilingExtraction)
   # result = generator(prompt)
   ```

---

## 3. Self-Correction & Error-Retry Loops

When constrained decoding is unavailable (e.g., standard local inference), implement a deterministic retry loop that feeds validation errors back to the model:

```python
import json
from pydantic import ValidationError

def extract_with_self_healing(llm_call_fn, prompt: str, schema_cls: type[BaseModel], max_retries: int = 3) -> BaseModel:
    """Executes extraction with programmatic error feedback on schema violations."""
    system_schema = json.dumps(schema_cls.model_json_schema(), indent=2)
    current_prompt = (
        f"{prompt}\n\n"
        f"You MUST reply with ONLY a single valid JSON object adhering strictly to this JSON Schema:\n"
        f"```json\n{system_schema}\n```"
    )
    
    for attempt in range(max_retries):
        raw_output = llm_call_fn(current_prompt)
        
        # Clean potential markdown fences
        clean_json = raw_output.strip()
        if clean_json.startswith("```"):
            clean_json = clean_json.split("\n", 1)[-1].rsplit("```", 1)[0].strip()
            
        try:
            data = json.loads(clean_json)
            validated = schema_cls.model_validate(data)
            return validated
        except (json.JSONDecodeError, ValidationError) as e:
            if attempt == max_retries - 1:
                raise RuntimeError(f"Extraction failed after {max_retries} attempts. Last error: {e}")
            
            # Formulate targeted correction message
            current_prompt = (
                f"Your previous output failed validation with the following error:\n{str(e)}\n\n"
                f"Previous invalid output was:\n{raw_output}\n\n"
                f"Please fix the validation errors and output the corrected JSON adhering to the schema."
            )
```

---

## 4. Few-Shot Schema Conditioning

Conditioning the model with high-quality, fully populated JSON examples in the prompt significantly reduces syntax hallucinations and schema drift.

### Few-Shot Template Structure
```text
System:
You are an expert financial analyst. Your task is to extract structured corporate events from news text.
Always conform to the provided JSON Schema.

Example Input:
"Alphabet Inc. reported Q3 2024 revenue of $88.27 billion, beating analyst forecasts of $86.3 billion."

Example Output:
{
  "company_name": "Alphabet Inc.",
  "ticker": "GOOGL",
  "filing_date": "2024-10-29",
  "report_type": "10-Q",
  "metrics": [
    {
      "metric_name": "revenue",
      "value": 88270000000.0,
      "unit": "USD",
      "fiscal_period": "Q3-2024",
      "confidence_score": 0.99
    }
  ],
  "key_risks": []
}

User:
Extract from the following document:
{INPUT_DOCUMENT}
```

---

## 5. Domain Document Extraction Checklist

1. **Unit & Scale Standardization**: Ensure prompt explicitly instructs model to convert scale words ("$88.27 billion" -> `88270000000.0`).
2. **Missing Field Strategy**: Explicitly set optional fields to `None` / `null` rather than generating placeholder strings like `"N/A"` or `"None"`.
3. **Chunking Large Filings**: Do not feed 200-page 10-K filings in a single prompt. Segment by Item sections (`Item 1A: Risk Factors`, `Item 7: MD&A`, `Item 8: Financial Statements`) and extract per-section sub-schemas before aggregating.
