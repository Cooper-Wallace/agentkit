---
name: feature-engineering
description: Systematic feature generation, interaction terms, cyclical encodings, and robust scaling for tabular and time-series data.
---

# Feature Engineering Standards & Patterns

This skill provides production standards for feature creation, transformations, robust scaling, and leakage-free encoding across tabular and sequential datasets.

---

## 1. Temporal Cyclical Encodings

Calendar and clock attributes (hour, day of week, month, minute) are cyclical: hour 23 is immediately adjacent to hour 0. Treating them as ordinal integers ($0, 1, 2, \dots, 23$) introduces artificial discontinuities at cycle boundaries.

### Trigonometric Sin/Cos Transformation
Map cyclical periods to two-dimensional Cartesian coordinates on the unit circle:
$$x_{\sin} = \sin\left(\frac{2\pi \cdot t}{T}\right), \quad x_{\cos} = \cos\left(\frac{2\pi \cdot t}{T}\right)$$
where $T$ is the period length.

```python
import numpy as np
import pandas as pd

def encode_cyclical_features(df: pd.DataFrame, timestamp_col: str) -> pd.DataFrame:
    ts = pd.to_datetime(df[timestamp_col])
    out = df.copy()
    
    # Hour of day (T = 24)
    out['hour_sin'] = np.sin(2 * np.pi * ts.dt.hour / 24.0)
    out['hour_cos'] = np.cos(2 * np.pi * ts.dt.hour / 24.0)
    
    # Day of week (T = 7, 0=Monday .. 6=Sunday)
    out['dow_sin'] = np.sin(2 * np.pi * ts.dt.dayofweek / 7.0)
    out['dow_cos'] = np.cos(2 * np.pi * ts.dt.dayofweek / 7.0)
    
    # Month of year (T = 12, 1..12 -> 0..11)
    out['month_sin'] = np.sin(2 * np.pi * (ts.dt.month - 1) / 12.0)
    out['month_cos'] = np.cos(2 * np.pi * (ts.dt.month - 1) / 12.0)
    
    # Day of year (T = 365.25)
    out['doy_sin'] = np.sin(2 * np.pi * (ts.dt.dayofyear - 1) / 365.25)
    out['doy_cos'] = np.cos(2 * np.pi * (ts.dt.dayofyear - 1) / 365.25)
    
    return out
```

*Rule*: Both sine and cosine components must be retained together. A single component cannot uniquely disambiguate points on opposite halves of the cycle.

---

## 2. Categorical Target Encoding with Out-Of-Fold (OOF) Regularization

Target encoding replaces categorical levels with the expected target value for that category. Naive target encoding causes catastrophic target leakage and severe overfitting.

### Regularized Smoothed Target Encoding
To eliminate leakage, use **Out-of-Fold (OOF)** estimation with Bayesian shrinkage smoothing:
$$\hat{S}_i = \frac{n_i \cdot \bar{y}_i + m \cdot \bar{y}_{\text{global}}}{n_i + m}$$
where:
- $n_i$ is the category count in training folds
- $\bar{y}_i$ is the category target mean in training folds
- $\bar{y}_{\text{global}}$ is the overall target mean
- $m$ is the smoothing weight parameter (shrinkage weight)

```python
from sklearn.model_selection import KFold

class OutOfFoldTargetEncoder:
    """Out-of-fold target encoder with Bayesian smoothing to prevent target leakage."""
    def __init__(self, n_splits: int = 5, smoothing: float = 10.0, noise_level: float = 0.01):
        self.n_splits = n_splits
        self.smoothing = smoothing
        self.noise_level = noise_level
        self.encoding_map_ = {}
        self.global_mean_ = 0.0

    def fit_transform(self, X: pd.Series, y: pd.Series) -> pd.Series:
        self.global_mean_ = y.mean()
        encoded = pd.Series(index=X.index, dtype=float)
        kf = KFold(n_splits=self.n_splits, shuffle=True, random_state=42)

        for train_idx, val_idx in kf.split(X, y):
            X_tr, y_tr = X.iloc[train_idx], y.iloc[train_idx]
            X_val = X.iloc[val_idx]
            
            stats = y_tr.groupby(X_tr).agg(['count', 'mean'])
            smoothed = (stats['count'] * stats['mean'] + self.smoothing * self.global_mean_) / (stats['count'] + self.smoothing)
            
            val_encoded = X_val.map(smoothed).fillna(self.global_mean_)
            if self.noise_level > 0:
                val_encoded += np.random.normal(0, self.noise_level * val_encoded.std(), size=len(val_encoded))
            encoded.iloc[val_idx] = val_encoded

        # Compute full training set encoding map for inference
        full_stats = y.groupby(X).agg(['count', 'mean'])
        self.encoding_map_ = ((full_stats['count'] * full_stats['mean'] + self.smoothing * self.global_mean_) / 
                              (full_stats['count'] + self.smoothing)).to_dict()
        return encoded

    def transform(self, X: pd.Series) -> pd.Series:
        return X.map(self.encoding_map_).fillna(self.global_mean_)
```

---

## 3. Rolling Aggregation Statistics

Multi-scale rolling statistics capture local trend, dispersion, asymmetry, and extremes across varying temporal horizons (e.g., 5, 20, 60 periods).

### Multi-Moment Window Aggregations
Compute moments across rolling windows:
1. **Location**: Rolling Mean $\mu_W$
2. **Dispersion**: Rolling Standard Deviation $\sigma_W$
3. **Asymmetry (Skewness)**: Captures upside vs downside tail risk
4. **Tail Heaviness (Kurtosis)**: Identifies outlier regimes
5. **Robust Order Statistics**: Rolling Median, 10th and 90th Percentiles, Interquartile Range ($\text{IQR} = Q_{75} - Q_{25}$)

```python
def generate_rolling_aggregations(df: pd.DataFrame, value_col: str, windows: list[int]) -> pd.DataFrame:
    res = pd.DataFrame(index=df.index)
    for w in windows:
        roll = df[value_col].rolling(window=w, min_periods=max(2, w // 2))
        res[f"{value_col}_roll_mean_{w}"] = roll.mean()
        res[f"{value_col}_roll_std_{w}"] = roll.std()
        res[f"{value_col}_roll_skew_{w}"] = roll.skew()
        res[f"{value_col}_roll_q10_{w}"] = roll.quantile(0.10)
        res[f"{value_col}_roll_q90_{w}"] = roll.quantile(0.90)
        res[f"{value_col}_roll_iqr_{w}"] = roll.quantile(0.75) - roll.quantile(0.25)
        # Rolling Z-score
        res[f"{value_col}_roll_zscore_{w}"] = (df[value_col] - res[f"{value_col}_roll_mean_{w}"]) / (res[f"{value_col}_roll_std_{w}"] + 1e-8)
    return res
```

---

## 4. Domain Interaction Terms & Ratios

Linear and tree-based models frequently struggle to learn multiplicative or division relationships natively. Explicitly engineer interaction terms:

### Domain Ratios & Relative Comparisons
- **Efficiency & Valuation Ratios**:
  $$\text{Ratio} = \frac{\text{Numerator}}{\text{Denominator} + \epsilon}$$
- **Log Ratios**: For strictly positive physical or financial quantities:
  $$\text{LogRatio} = \ln(\text{Val}_A) - \ln(\text{Val}_B)$$
- **Cohort Normalization (Difference from Group Benchmark)**:
  $$\Delta_{\text{cohort}} = x_i - \bar{x}_{\text{cohort}}, \quad z_{\text{cohort}} = \frac{x_i - \bar{x}_{\text{cohort}}}{\sigma_{\text{cohort}}}$$

```python
def build_interaction_features(df: pd.DataFrame) -> pd.DataFrame:
    out = df.copy()
    eps = 1e-7
    # Volume to volatility ratio
    if 'volume' in out.columns and 'volatility' in out.columns:
        out['volume_per_volatility'] = out['volume'] / (out['volatility'] + eps)
    # Log-relative price spreads
    if 'high' in out.columns and 'low' in out.columns:
        out['high_low_log_spread'] = np.log(out['high'] + eps) - np.log(out['low'] + eps)
    return out
```

---

## 5. Missing Value Imputation Strategies

Different data types require strictly defined imputation protocols:

### Time-Series / Sequential Imputation
- **Forward-Fill with Boundary Limit**: `df.ffill(limit=K)`
  - *Constraint*: Never perform backward fill (`bfill`) on features used in real-time scoring, as backward filling incorporates future observations.
- **Missingness Indicator**: Always append a binary flag column indicating whether the original value was missing:
  ```python
  df['feature_was_missing'] = df['feature'].isna().astype(int)
  ```
  Missingness itself is frequently an informative signal (e.g., missing metrics in scouting or trade logs).

### Cross-Sectional / Tabular Imputation
- Use **Median Imputation** for skewed numerical distributions with extreme outliers.
- Use **Iterative / KNN Imputer** fitted strictly on the training partition when feature correlations are strong.
- For categorical features, designate a distinct `'__MISSING__'` category token rather than mode imputation.

---

## 6. Robust Feature Scaling & Normalization

| Scaler | Transformation Formula | Best For | Sensitivity to Outliers |
| :--- | :--- | :--- | :--- |
| **StandardScaler** | $z = \frac{x - \mu}{\sigma}$ | Gaussian features, linear models, neural nets | High (mean & std corrupted by extreme tails) |
| **RobustScaler** | $z = \frac{x - \text{median}}{\text{IQR}}$ | Heavy-tailed financial bars, player stats | Low (bounded by 25th-75th percentiles) |
| **QuantileTransformer** | $u = F(x) \in [0, 1]$ | Highly skewed, multi-modal, or power-law features | Non-sensitive (maps rank to uniform/normal) |
| **PowerTransformer** | Yeo-Johnson / Box-Cox | Stabilizing variance and minimizing skew | Moderate |

### Fit-Transform Boundary Discipline
```python
from sklearn.preprocessing import RobustScaler

# CORRECT:
scaler = RobustScaler()
X_train_scaled = scaler.fit_transform(X_train)
X_test_scaled = scaler.transform(X_test)  # Strictly transform only!

# INCORRECT (LEAKAGE):
# X_all_scaled = scaler.fit_transform(pd.concat([X_train, X_test]))
```
