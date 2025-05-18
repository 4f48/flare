use axum::{
    Router,
    extract::{
        ConnectInfo, MatchedPath, WebSocketUpgrade,
        ws::{Message, WebSocket},
    },
    http::Request,
    response::IntoResponse,
    routing::get,
};
use axum_extra::TypedHeader;
use clap::Parser;
use std::{net::SocketAddr, ops::ControlFlow};
use tokio::net::TcpListener;
use tower_http::trace::TraceLayer;
use tracing::{debug, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
struct Args {
    #[arg(short, long, default_value = "8080")]
    port: u16,
}

#[tokio::main]
async fn main() {
    let args = Args::parse();

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

    let app =
        Router::new()
            .route("/", get(handler))
            .layer(
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
            );

    let listener = TcpListener::bind(format!("0.0.0.0:{}", args.port))
        .await
        .unwrap();
    info!("listening on {}", listener.local_addr().unwrap());
    axum::serve(
        listener,
        app.into_make_service_with_connect_info::<SocketAddr>(),
    )
    .await
    .unwrap();
}

async fn handler(
    ws: WebSocketUpgrade,
    user_agent: Option<TypedHeader<headers::UserAgent>>,
    ConnectInfo(addr): ConnectInfo<SocketAddr>,
) -> impl IntoResponse {
    let user_agent = if let Some(TypedHeader(user_agent)) = user_agent {
        user_agent.to_string()
    } else {
        String::from("unknown")
    };
    debug!("{user_agent} connected from {addr}");
    ws.on_upgrade(move |socket| handle_socket(socket, addr))
}

async fn handle_socket(mut socket: WebSocket, addr: SocketAddr) {
    if let Some(msg) = socket.recv().await {
        if let Ok(msg) = msg {
            if process_message(msg, addr, socket).await.is_break() {}
        } else {
            println!("client {addr} abruptly disconnected");
        }
    }
}

async fn process_message(
    msg: Message,
    addr: SocketAddr,
    mut socket: WebSocket,
) -> ControlFlow<(), ()> {
    match msg {
        Message::Text(t) => {
            info!("{addr} > {t}");
            socket.send(Message::text(t)).await.unwrap_or(());
        }
        Message::Close(c) => {
            if let Some(cf) = c {
                debug!(
                    "{addr} is disconnecting with code {} and reason \"{}\"",
                    cf.code, cf.reason
                );
            } else {
                debug!("{addr} is disconnecting")
            }
            return ControlFlow::Break(());
        }
        _ => (),
    }
    ControlFlow::Continue(())
}
