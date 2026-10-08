use thiserror::Error;

/// Et vedtak lest fra en linje i filen fra fagsystemet.
#[derive(Debug, PartialEq)]
pub struct Vedtak {
    pub id: String,
    pub belop: u32,
}

#[derive(Debug, Error, PartialEq)]
pub enum VedtakFeil {
    #[error("linjen mangler id")]
    ManglerId,
    #[error("linjen mangler beløp")]
    ManglerBelop,
    #[error("ugyldig beløp: {0}")]
    UgyldigBelop(String),
}

/// Leser en linje på formen "id;belop".
pub fn les(linje: &str) -> Result<Vedtak, VedtakFeil> {
    let mut deler = linje.split(';');
    let id = deler.next().filter(|s| !s.is_empty()).ok_or(VedtakFeil::ManglerId)?;
    let belop = deler.next().ok_or(VedtakFeil::ManglerBelop)?;
    let belop = belop.parse().map_err(|_| VedtakFeil::UgyldigBelop(belop.to_string()))?;
    Ok(Vedtak { id: id.to_string(), belop })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn leser_gyldig_linje() {
        assert_eq!(les("v1;250"), Ok(Vedtak { id: "v1".into(), belop: 250 }));
    }

    #[test]
    fn mangler_id() {
        assert_eq!(les(";1"), Err(VedtakFeil::ManglerId));
    }

    #[test]
    fn mangler_belop() {
        assert_eq!(les("v1"), Err(VedtakFeil::ManglerBelop));
    }

    #[test]
    fn ugyldig_belop() {
        assert_eq!(les("v1;x"), Err(VedtakFeil::UgyldigBelop("x".into())));
    }
}
