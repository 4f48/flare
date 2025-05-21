mod diceware;
mod types;
mod websocket;

use axum::{Router, extract::MatchedPath, http::Request, routing::get};
use clap::Parser;
use dashmap::DashMap;
use diceware::{generate_code, read_word_list};
use std::sync::Arc;
use std::{net::SocketAddr, time::Duration};
use tokio::net::TcpListener;
use tokio::signal;
use tower_http::{timeout::TimeoutLayer, trace::TraceLayer};
use tracing::info;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};
use types::{AppState, Args, Sessions};

#[tokio::main]
async fn main() {
    let args = Args::parse();

    let sessions: Sessions = Arc::new(DashMap::new());
    let words = read_word_list("eff_large_wordlist.txt").expect("couldn't parse wordlist");

    let state = AppState {
        sessions: sessions.clone(),
        wordlist: words,
    };

    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env().unwrap_or_else(|_| {
                format!(
                    "{}=debug,tower_http=debug,axum::rejection=trace",
                    env!("CARGO_CRATE_NAME")
                )
                .into()
            }),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    let app = Router::new()
        .route("/", get(websocket::handler))
        .with_state(state)
        .layer((
            TraceLayer::new_for_http().make_span_with(|request: &Request<_>| {
                let matched_path = request
                    .extensions()
                    .get::<MatchedPath>()
                    .map(MatchedPath::as_str);

                tracing::info_span!(
                    "http_request",
                    method = ?request.method(),
                    matched_path,
                    some_other_field = tracing::field::Empty,
                )
            }),
            TimeoutLayer::new(Duration::from_secs(60)),
        ));

    let listener = TcpListener::bind(format!("0.0.0.0:{}", args.port))
        .await
        .unwrap();
    info!("listening on {}", listener.local_addr().unwrap());
    axum::serve(
        listener,
        app.into_make_service_with_connect_info::<SocketAddr>(),
    )
    .with_graceful_shutdown(shutdown_signal())
    .await
    .unwrap();
}

async fn shutdown_signal() {
    let ctrl_c = async {
        signal::ctrl_c()
            .await
            .expect("failed to install ctrl+c handler");
    };

    #[cfg(unix)]
    let terminate = async {
        signal::unix::signal(signal::unix::SignalKind::terminate())
            .expect("failed to install signal handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {
            info!("Shutting down...");
        },
        _ = terminate => {
            info!("Shutting down...");
        }
    }
}
