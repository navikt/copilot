// Evaluator-owned: never copied into the agent's workspace.
use vedtak_parser::{les, VedtakFeil};

#[test]
fn gyldig() {
    let v = les("v1;250").unwrap();
    assert_eq!((v.id.as_str(), v.belop), ("v1", 250));
}

#[test]
fn mangler_belop() {
    assert!(matches!(les("v1"), Err(VedtakFeil::ManglerBelop)));
}

#[test]
fn ugyldig_belop() {
    match les("v1;abc") {
        Err(e @ VedtakFeil::UgyldigBelop(_)) => {
            assert!(matches!(&e, VedtakFeil::UgyldigBelop(s) if s == "abc"));
            assert!(e.to_string().contains("abc"), "{e}");
        }
        other => panic!("{other:?}"),
    }
}

#[test]
fn mangler_id() {
    assert!(matches!(les(";10"), Err(VedtakFeil::ManglerId)));
}

#[test]
fn er_en_std_error() {
    let e: Box<dyn std::error::Error> = Box::new(les("v1").unwrap_err());
    assert!(!e.to_string().is_empty());
}
